package main

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/TaitoUnited/fsnotify"
	log "github.com/sirupsen/logrus"
)

type MissionControl struct {
	Watcher   *fsnotify.Watcher
	Launchpad Launchpad
	Routes    []Route
	Enroute   *sync.WaitGroup
}

func NewMissionControl(retry int, shuttlesPath string) MissionControl {
	launchpad := NewLaunchpad(retry, shuttlesPath)

	return MissionControl{
		Watcher:   nil,
		Launchpad: launchpad,
		Routes:    []Route{},
	}
}

func (mc *MissionControl) Reload(base string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	routes, err := mc.DiscoverRoutes(base)
	if err != nil {
		return err
	}

	// Add watchers for all the route folders
	for _, route := range routes {
		if err := watcher.Add(route.Path); err != nil {
			return err
		}
	}

	start := time.Now()
	log.Info("Mission control critical section starts")

	// Close the old watcher if it exists
	if mc.Watcher != nil {
		if err := mc.Watcher.Close(); err != nil {
			return err
		}
	}

	// Replace routes and start a new watcher
	mc.Routes = routes
	go mc.Watch(watcher)

	log.WithFields(log.Fields{
		"elapsed": time.Since(start),
	}).Info("Mission control critical section ends")

	return nil
}

func (mc *MissionControl) Watch(watcher *fsnotify.Watcher) {
	mc.Watcher = watcher
	defer mc.Watcher.Close()

	for {
		select {
		case event := <-mc.Watcher.Events:
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

			directory := path.Dir(event.Name)

			found := false
			for _, route := range mc.Routes {
				if route.Path == directory {
					mc.Launchpad.AddShuttle(NewShuttle(route, path.Base(event.Name)))

					found = true
					break
				}
			}

			if !found {
				log.WithFields(log.Fields{
					"path": event.Name,
				}).Error("Failed to find route for fsnotify event")
			}

		case err := <-mc.Watcher.Errors:
			// nil errors caused by a closed watcher
			if err != nil {
				log.WithFields(log.Fields{
					"err": err,
				}).Error("fsnotify error")
			}
		}
	}
}

func (mc MissionControl) DiscoverRoutes(base string) ([]Route, error) {
	routes := []Route{}

	files, err := ioutil.ReadDir(base)
	if err != nil {
		return routes, err
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		endpointPath := path.Join(base, file.Name(), ".endpoint")
		if _, err := os.Stat(endpointPath); err == nil {
			endpoint, err := ioutil.ReadFile(endpointPath)
			if err != nil {
				return routes, err
			}

			route := Route{
				Path:     path.Join(base, file.Name(), "files"),
				Endpoint: strings.TrimSpace(string(endpoint)),
			}

			routes = append(routes, route)
		}
	}

	return routes, nil
}
