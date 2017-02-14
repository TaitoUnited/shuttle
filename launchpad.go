package main

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/TaitoUnited/fsnotify"
	log "github.com/sirupsen/logrus"
)

type Launchpad struct {
	Watcher  *fsnotify.Watcher
	Queue    chan Shuttle
	Shuttles []Shuttle
	Routes   []Route
}

func NewLaunchpad() Launchpad {
	return Launchpad{
		Watcher:  nil,
		Queue:    make(chan Shuttle, 100),
		Shuttles: []Shuttle{},
		Routes:   []Route{},
	}
}

func (p *Launchpad) HasShuttle(shuttle Shuttle) bool {
	found := false
	for _, otherShuttle := range p.Shuttles {
		if shuttle.Route.Path == otherShuttle.Route.Path {
			found = true
			break
		}
	}

	return found
}

func (p *Launchpad) AddShuttle(shuttle Shuttle) {
	if p.HasShuttle(shuttle) {
		log.WithFields(log.Fields{
			"shuttle": shuttle,
		}).Info("Refused to add duplicate shuttle")
		return
	}

	log.WithFields(log.Fields{
		"shuttle": shuttle,
	}).Info("Adding shuttle")

	log.WithFields(log.Fields{
		"shuttles": p.Shuttles,
	}).Info("Before")
	p.Shuttles = append(p.Shuttles, shuttle)
	log.WithFields(log.Fields{
		"shuttles": p.Shuttles,
	}).Info("After")
	p.Queue <- shuttle
}

func (p *Launchpad) RemoveShuttle(shuttle Shuttle) {
	log.WithFields(log.Fields{
		"shuttle": shuttle,
	}).Info("Removing shuttle")

	shuttles := []Shuttle{}
	for _, otherShuttle := range p.Shuttles {
		if shuttle.Route.Path != otherShuttle.Route.Path {
			shuttles = append(shuttles, otherShuttle)
		}
	}

	p.Shuttles = shuttles
}

func (p *Launchpad) Reload(base string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	routes, err := p.DiscoverRoutes(base)
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
	log.Info("Launchpad critical section starts")

	// Close the old watcher if it exists
	if p.Watcher != nil {
		if err := p.Watcher.Close(); err != nil {
			return err
		}
	}

	// Replace routes and start a new watcher
	p.Routes = routes
	go p.Watch(watcher)

	log.WithFields(log.Fields{
		"elapsed": time.Since(start),
	}).Info("Launchpad critical section ends")

	return nil
}

func (p *Launchpad) LaunchShuttles() {
	for shuttle := range p.Queue {
		log.WithFields(log.Fields{
			"shuttle":  shuttle,
			"shuttles": p.Shuttles,
		}).Info("Received shuttle")

		if !p.HasShuttle(shuttle) {
			// Discard the shuttle from queue
			log.WithFields(log.Fields{
				"shuttle":  shuttle,
				"shuttles": p.Shuttles,
			}).Info("Shuttle gone missing")

			continue
		}

		logger := log.WithFields(log.Fields{
			"path":     shuttle.Route.Path,
			"endpoint": shuttle.Route.Endpoint,
			"filename": shuttle.Filename,
		})

		logger.Info("Shuttle received, transporting to destination")

		if err := shuttle.Route.Transport(shuttle.Filename); err != nil {
			logger.WithFields(log.Fields{
				"err": err,
			}).Error("Shuttle crashed, retrying in 5 seconds")

			go func(queue chan Shuttle, shuttle Shuttle) {
				time.Sleep(5 * time.Second)
				queue <- shuttle
			}(p.Queue, shuttle)

			continue
		}

		p.RemoveShuttle(shuttle)
		logger.Info("Shuttle arrived at the destination successfully")
	}
}

func (p *Launchpad) Watch(watcher *fsnotify.Watcher) {
	p.Watcher = watcher
	defer p.Watcher.Close()

	for {
		select {
		case event := <-p.Watcher.Events:
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
			for _, route := range p.Routes {
				if route.Path == directory {
					p.AddShuttle(NewShuttle(route, path.Base(event.Name)))

					found = true
					break
				}
			}

			if !found {
				log.WithFields(log.Fields{
					"path": event.Name,
				}).Error("Failed to find route for fsnotify event")
			}

		case err := <-p.Watcher.Errors:
			// nil errors caused by a closed watcher
			if err != nil {
				log.WithFields(log.Fields{
					"err": err,
				}).Error("fsnotify error")
			}
		}
	}
}

func (p Launchpad) DiscoverRoutes(base string) ([]Route, error) {
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
