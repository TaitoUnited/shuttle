package main

import (
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/TaitoUnited/fsnotify"
	log "github.com/sirupsen/logrus"
)

// LocalService monitors the given routes using fsnotify.
// LocalService is a special service that is mutually exclusive with other services
// because a IN_CLOSE_WRITE event might not be the end of a transfer in other services.
type LocalService struct {
	routes             []Route
	chroot             string
	writeNotifications chan WriteNotification
	watcher            *fsnotify.Watcher
}

// NewLocalService creates a new LocalService.
func NewLocalService(chroot string, routes []Route) *LocalService {
	return &LocalService{
		routes:             routes,
		chroot:             chroot,
		writeNotifications: make(chan WriteNotification, 100),
	}
}

// Name returns the name of the service.
func (s *LocalService) Name() string {
	return "local"
}

// Start starts the service.
func (s *LocalService) Start() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	for _, route := range s.routes {
		path := filepath.Join(s.chroot, route.Username)
		if err := watcher.Add(path); err != nil {
			return err
		}
	}

	s.watcher = watcher

	go s.watch()

	return nil
}

// Reload reloads the service using provided new routes.
func (s *LocalService) Reload(routes []Route) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	for _, route := range routes {
		path := filepath.Join(s.chroot, route.Username)
		if err := watcher.Add(path); err != nil {
			return err
		}
	}

	oldWatcher := s.watcher
	s.watcher = watcher

	return oldWatcher.Close()
}

// Stop stops the server gracefully.
func (s *LocalService) Stop() error {
	// This is really nasty but it works
	// Wait for the channels to be drained
	for {
		writeNotifications := reflect.ValueOf(s.writeNotifications)

		if writeNotifications.Len() == 0 {
			break
		}

		time.Sleep(1 * time.Second)
	}

	close(s.writeNotifications)

	if err := s.watcher.Close(); err != nil {
		return err
	}

	return nil
}

// WriteNotifications returns the file write notification channel.
func (s *LocalService) WriteNotifications() chan WriteNotification {
	return s.writeNotifications
}

func (s *LocalService) watch() {
	for {
		select {
		case event := <-s.watcher.Events:
			if event.Op&fsnotify.CloseWrite != fsnotify.CloseWrite {
				continue
			}

			fileinfo, err := os.Stat(event.Name)
			if err != nil {
				log.WithFields(log.Fields{
					"path": event.Name,
				}).Error("Failed to stat on fsnotify event")
				continue
			}

			if fileinfo.IsDir() {
				continue
			}

			username := filepath.Base(filepath.Dir(event.Name))
			s.writeNotifications <- WriteNotification{
				Username: username,
				Path:     event.Name,
			}

		case err := <-s.watcher.Errors:
			// nil errors caused by a closed watcher
			if err != nil {
				log.WithFields(log.Fields{
					"err": err,
				}).Error("fsnotify error")
			}
		}
	}
}
