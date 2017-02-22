package main

import (
	"fmt"
	"io"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/AntiPaste/sftp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type SftpService struct {
	routes             []Route
	routesMutex        *sync.RWMutex
	privateKey         ssh.Signer
	host               string
	port               int
	chroot             string
	incoming           chan sftp.WrittenFile
	writeNotifications chan WriteNotification
	listener           net.Listener
	servers            map[string]*sftp.Server
	quit               chan bool
}

func NewSftpService(host string, port int, chroot string, routes []Route, privateKey ssh.Signer) *SftpService {
	return &SftpService{
		routes:             routes,
		routesMutex:        &sync.RWMutex{},
		privateKey:         privateKey,
		host:               host,
		port:               port,
		chroot:             chroot,
		incoming:           make(chan sftp.WrittenFile, 100),
		writeNotifications: make(chan WriteNotification, 100),
		servers:            make(map[string]*sftp.Server),
		quit:               make(chan bool, 1),
	}
}

func (s *SftpService) Name() string {
	return "sftp"
}

func (s *SftpService) Start() error {
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			s.routesMutex.RLock()
			defer s.routesMutex.RUnlock()

			for _, route := range s.routes {
				if c.User() == route.Username && string(pass) == route.Password {
					return nil, nil
				}
			}

			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}

	config.AddHostKey(s.privateKey)

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.host, s.port))
	if err != nil {
		return err
	}

	s.listener = listener

	go s.accept(config)
	go s.watchIncoming()

	return nil
}

func (s *SftpService) Reload(routes []Route) error {
	s.routesMutex.Lock()
	defer s.routesMutex.Unlock()

	s.routes = routes
	return nil
}

func (s *SftpService) Stop() error {
	s.quit <- true

	if err := s.listener.Close(); err != nil {
		return err
	}

	for _, server := range s.servers {
		if err := server.Stop(); err != nil {
			return err
		}
	}

	// This is really nasty but it works
	// Wait for the channels to be drained
	for {
		writeNotifications := reflect.ValueOf(s.writeNotifications)
		incoming := reflect.ValueOf(s.incoming)

		if writeNotifications.Len() == 0 && incoming.Len() == 0 {
			break
		}

		time.Sleep(1 * time.Second)
	}

	close(s.incoming)
	close(s.writeNotifications)

	return nil
}

func (s *SftpService) WriteNotifications() chan WriteNotification {
	return s.writeNotifications
}

func (s *SftpService) accept(config *ssh.ServerConfig) {
	for {
		newConn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
			}

			log.WithFields(log.Fields{
				"err": err,
			}).Error("Failed to accept incoming SSH connection")
			continue
		}

		// Before use, a handshake must be performed on the incoming net.Conn.
		serverConn, chans, reqs, err := ssh.NewServerConn(newConn, config)
		if err != nil {
			if err != io.EOF {
				log.WithFields(log.Fields{
					"err": err,
				}).Error("Failed to handshake SSH connection", err)
			}

			continue
		}

		go s.handleClient(serverConn, chans, reqs)
	}
}

func (s *SftpService) handleClient(serverConn *ssh.ServerConn, chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request) {
	defer serverConn.Close()

	// The incoming Request channel must be serviced.
	go ssh.DiscardRequests(reqs)

	serverID := string(serverConn.SessionID())

	// Service the incoming Channel channel.
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of an SFTP session, this is "subsystem"
		// with a payload string of "<length=4>sftp"
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Could not accept channel")
			break
		}

		// Sessions have out-of-band requests such as "shell",
		// "pty-req" and "env".  Here we handle only the
		// "subsystem" request.
		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := false
				switch req.Type {
				case "subsystem":
					if string(req.Payload[4:]) == "sftp" {
						ok = true
					}
				}

				req.Reply(ok, nil)
			}
		}(requests)

		serverOptions := []sftp.ServerOption{
			sftp.Chroot(s.chroot),
			sftp.NotifyWrite(s.incoming),
			sftp.AsUser(serverConn.User()),
		}

		server, err := sftp.NewServer(channel, serverOptions...)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Failed to create new SFTP server instance")
			break
		}

		s.servers[serverID] = server

		if err := server.Serve(); err != nil {
			if err != io.EOF {
				log.WithFields(log.Fields{
					"err": err,
				}).Error("SFTP server instance crashed")
			}

			break
		}
	}

	delete(s.servers, serverID)
}

func (s *SftpService) watchIncoming() {
	for writtenFile := range s.incoming {
		notification := WriteNotification{
			Username: writtenFile.User,
			Path:     writtenFile.Path,
		}

		s.writeNotifications <- notification
	}
}
