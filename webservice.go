package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// WebService is a web server.
type WebService struct {
	routes             []Route
	host               string
	port               int
	insecurePort       int
	chroot             string
	certificate        tls.Certificate
	writeNotifications chan WriteNotification
	server             *http.Server
	insecureServer     *http.Server
	rootTemplate       *template.Template
}

type userFile struct {
	Mode     string
	Modified string
	Name     string
}

// NewWebService creates a new WebService.
func NewWebService(host string, port int, insecurePort int, chroot string, certificate tls.Certificate, routes []Route) *WebService {
	return &WebService{
		routes:             routes,
		host:               host,
		port:               port,
		insecurePort:       insecurePort,
		chroot:             chroot,
		certificate:        certificate,
		writeNotifications: make(chan WriteNotification, 100),
		rootTemplate:       template.Must(template.New("root").Parse(rootTemplateSource)),
	}
}

// Name returns the name of the service.
func (s *WebService) Name() string {
	return "web"
}

// Start starts the service.
func (s *WebService) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.auth(s.serveRoot))
	mux.HandleFunc("/download", s.auth(s.serveFile))
	mux.HandleFunc("/upload", s.auth(s.handleUpload))

	tlsConfig := &tls.Config{
		Certificates:             []tls.Certificate{s.certificate},
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.host, s.port),
		Handler:      mux,
		TLSConfig:    tlsConfig,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
	}

	s.insecureServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.host, s.insecurePort),
		Handler: http.HandlerFunc(s.httpRedirect),
	}

	go s.server.ListenAndServeTLS("", "")
	go s.insecureServer.ListenAndServe()

	return nil
}

// Reload reloads the service using provided new routes.
func (s *WebService) Reload(routes []Route) error {
	s.routes = routes
	return nil
}

// Stop stops the server gracefully.
func (s *WebService) Stop() error {
	// This is not crucial so errors are ignored
	s.insecureServer.Close()

	return s.server.Shutdown(context.Background())
}

// WriteNotifications returns the file write notification channel.
func (s *WebService) WriteNotifications() chan WriteNotification {
	return s.writeNotifications
}

func (s *WebService) auth(handler http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		username, password, _ := request.BasicAuth()
		for _, route := range s.routes {
			if route.Username == username {
				if err := bcrypt.CompareHashAndPassword([]byte(route.Password), []byte(password)); err == nil {
					handler(writer, request)
					return
				}
			}
		}

		writer.Header().Set("WWW-Authenticate", `Basic realm="Shuttle"`)
		http.Error(writer, "Unauthorized.", http.StatusUnauthorized)
	}
}

func (s *WebService) httpRedirect(writer http.ResponseWriter, request *http.Request) {
	host, _, _ := net.SplitHostPort(request.Host)
	if s.port != 443 {
		host += fmt.Sprintf(":%d", s.port)
	}

	redirectURL := fmt.Sprintf("https://%s%s", host, request.URL.Path)
	http.Redirect(writer, request, redirectURL, http.StatusMovedPermanently)
}

func (s *WebService) serveRoot(writer http.ResponseWriter, request *http.Request) {
	username, _, _ := request.BasicAuth()

	path := filepath.Join(s.chroot, username)

	files, err := ioutil.ReadDir(path)
	if err != nil {
		http.Error(writer, "Error while reading directory", http.StatusInternalServerError)
		return
	}

	userFiles := []userFile{}
	for _, file := range files {
		var timestamp string
		if file.ModTime().Year() != time.Now().Year() {
			timestamp = file.ModTime().Format("Jan 2 2006")
		} else {
			timestamp = file.ModTime().Format("Jan 2 15:04")
		}

		userFiles = append(userFiles, userFile{
			Mode:     file.Mode().String(),
			Modified: timestamp,
			Name:     file.Name(),
		})
	}

	if err := s.rootTemplate.Execute(writer, userFiles); err != nil {
		http.Error(writer, "Templating error", http.StatusInternalServerError)
		return
	}
}

func (s *WebService) serveFile(writer http.ResponseWriter, request *http.Request) {
	username, _, _ := request.BasicAuth()

	filename := request.URL.Query().Get("filename")
	filename = filepath.Base(filename)

	path := filepath.Join(s.chroot, username, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.Error(writer, "File not found", http.StatusNotFound)
		return
	}

	file, err := os.Open(path)
	if err != nil {
		http.Error(writer, "File open failed", http.StatusInternalServerError)
		return
	}

	defer file.Close()

	contentType, err := DetectContentType(file)
	if err != nil {
		http.Error(writer, "File read failed", http.StatusInternalServerError)
		return
	}

	file.Seek(0, 0)

	writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	writer.Header().Set("Content-Type", contentType)

	io.Copy(writer, file)
}

func (s *WebService) handleUpload(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "POST" {
		http.Error(writer, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	request.ParseMultipartForm(1024000000)

	incoming, handler, err := request.FormFile("file")
	if err != nil {
		http.Error(writer, "Upload error", http.StatusInternalServerError)
		return
	}

	defer incoming.Close()

	username, _, _ := request.BasicAuth()
	path := filepath.Join(s.chroot, username, handler.Filename)

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		http.Error(writer, "Upload error", http.StatusInternalServerError)
		return
	}

	defer file.Close()

	io.Copy(file, incoming)

	s.writeNotifications <- WriteNotification{
		Username: username,
		Path:     path,
	}

	http.Redirect(writer, request, "/", http.StatusFound)
}

const rootTemplateSource = `
	<table>
		<thead>
			<tr>
				<th>Mode</th>
				<th>Modified</th>
				<th>Name</th>
				<th>Download</th>
			</tr>
		</thead>
		<tbody>
			{{range .}}
				<tr>
					<td>{{.Mode}}</td>
					<td>{{.Modified}}</td>
					<td>{{.Name}}</td>
					<td><a href="/download?filename={{.Name}}">Download</a>
				</tr>
			{{end}}
		</tbody>
	</table>

	<br />

	<form action="/upload" method="post" enctype="multipart/form-data">
		<label>Select a file to upload</label><br />
		<input type="file" name="file" />
		<input type="submit" value="Upload" />
	</form>
`
