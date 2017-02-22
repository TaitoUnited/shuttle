package main

import (
	"crypto/tls"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/AntiPaste/ftpserver/server"
	log "github.com/sirupsen/logrus"
)

// FtpService is a FTP server.
type FtpService struct {
	routes             []Route
	host               string
	port               int
	chroot             string
	writeNotifications chan WriteNotification
	server             *server.FtpServer
	driver             *ftpDriver
}

// NewFtpService creates a new FtpService.
func NewFtpService(host string, port int, chroot string, routes []Route) *FtpService {
	return &FtpService{
		routes:             routes,
		host:               host,
		port:               port,
		chroot:             chroot,
		writeNotifications: make(chan WriteNotification, 100),
	}
}

// Name returns the name of the service.
func (s *FtpService) Name() string {
	return "ftp"
}

// Start starts the service.
func (s *FtpService) Start() error {
	s.driver = &ftpDriver{
		base:        s.chroot,
		routes:      s.routes,
		routesMutex: &sync.RWMutex{},
		settings: &server.Settings{
			ListenHost: s.host,
			ListenPort: s.port,
		},
		writeNotifications: s.WriteNotifications(),
	}

	s.server = server.NewFtpServer(s.driver)

	go s.serve()

	return nil
}

// Reload reloads the service using provided new routes.
func (s *FtpService) Reload(routes []Route) error {
	s.driver.SetRoutes(routes)
	return nil
}

// Stop stops the server gracefully.
func (s *FtpService) Stop() error {
	return s.server.Stop()
}

// WriteNotifications returns the file write notification channel.
func (s *FtpService) WriteNotifications() chan WriteNotification {
	return s.writeNotifications
}

func (s *FtpService) serve() {
	for {
		if err := s.server.ListenAndServe(); err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("FTP server crashed, restarting after 5 seconds")

			time.Sleep(5 * time.Second)
			continue
		}

		break
	}
}

// Define the ftpDriver that is used by the FtpService.
type ftpDriver struct {
	base               string
	settings           *server.Settings
	writeNotifications chan WriteNotification
	routes             []Route
	routesMutex        *sync.RWMutex
}

func (drv *ftpDriver) path(cc server.ClientContext, p string) string {
	chroot := filepath.Join(drv.base, cc.User())

	c := filepath.Clean(p)
	if filepath.IsAbs(c) && filepath.HasPrefix(c, chroot) {
		return c
	}
	return filepath.Join(chroot, c)
}

func (drv *ftpDriver) SetRoutes(routes []Route) {
	drv.routesMutex.Lock()
	defer drv.routesMutex.Unlock()

	drv.routes = routes
}

func (drv *ftpDriver) WelcomeUser(cc server.ClientContext) (string, error) {
	return "Shuttle", nil
}

func (drv *ftpDriver) AuthUser(cc server.ClientContext, user, pass string) (server.ClientHandlingDriver, error) {
	drv.routesMutex.RLock()
	defer drv.routesMutex.RUnlock()

	for _, route := range drv.routes {
		if route.Username == user && route.Password == pass {
			return drv, nil
		}
	}

	return nil, errors.New("Login incorrect")
}

// TODO
func (drv *ftpDriver) GetTLSConfig() (*tls.Config, error) {
	return &tls.Config{}, nil
}

func (drv *ftpDriver) ChangeDirectory(cc server.ClientContext, directory string) error {
	fileinfo, err := os.Stat(drv.path(cc, directory))
	if err != nil {
		return err
	}

	if !fileinfo.IsDir() {
		return errors.New("Destination is not a directory")
	}

	return nil
}

func (drv *ftpDriver) MakeDirectory(cc server.ClientContext, directory string) error {
	return os.Mkdir(drv.path(cc, directory), 0777)
}

func (drv *ftpDriver) ListFiles(cc server.ClientContext) ([]os.FileInfo, error) {
	files, err := ioutil.ReadDir(drv.path(cc, cc.Path()))
	if err != nil {
		return files, err
	}

	return files, nil
}

func (drv *ftpDriver) UserLeft(cc server.ClientContext) {}

func (drv *ftpDriver) OpenFile(cc server.ClientContext, path string, flag int) (server.FileStream, error) {
	// If we are writing and we are not in append mode, we should remove the file
	if (flag & os.O_WRONLY) != 0 {
		flag |= os.O_CREATE
		if (flag & os.O_APPEND) == 0 {
			// Ignore error, not crucial
			os.Remove(drv.path(cc, path))
		}
	}

	return os.OpenFile(drv.path(cc, path), flag, 0666)
}

func (drv *ftpDriver) GetFileInfo(cc server.ClientContext, path string) (os.FileInfo, error) {
	return os.Stat(drv.path(cc, path))
}

func (drv *ftpDriver) CanAllocate(cc server.ClientContext, size int) (bool, error) {
	var stat syscall.Statfs_t
	syscall.Statfs(drv.base, &stat)

	available := stat.Bavail * uint64(stat.Bsize)
	if available < uint64(size) {
		return false, nil
	}

	return true, nil
}

func (drv *ftpDriver) ChmodFile(cc server.ClientContext, path string, mode os.FileMode) error {
	return os.Chmod(drv.path(cc, path), mode)
}

func (drv *ftpDriver) DeleteFile(cc server.ClientContext, path string) error {
	return os.Remove(drv.path(cc, path))
}

func (drv *ftpDriver) RenameFile(cc server.ClientContext, from, to string) error {
	return os.Rename(drv.path(cc, from), drv.path(cc, to))
}

func (drv *ftpDriver) GetSettings() *server.Settings {
	return drv.settings
}

func (drv *ftpDriver) NotifyWrite(cc server.ClientContext, path string) error {
	drv.writeNotifications <- WriteNotification{
		Username: drv.path(cc, path),
		Path:     cc.User(),
	}

	return nil
}