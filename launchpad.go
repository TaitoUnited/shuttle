package main

import (
	"encoding/gob"
	"fmt"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Launchpad struct {
	Queue         chan Shuttle
	Retry         int
	Enroute       *sync.WaitGroup
	ShuttlesPath  string
	Shuttles      []Shuttle
	ShuttlesMutex *sync.Mutex
}

func NewLaunchpad(retry int, shuttlesPath string) Launchpad {
	return Launchpad{
		Queue:         make(chan Shuttle, 100),
		Retry:         retry,
		Enroute:       &sync.WaitGroup{},
		ShuttlesPath:  shuttlesPath,
		Shuttles:      []Shuttle{},
		ShuttlesMutex: &sync.Mutex{},
	}
}

func (lp *Launchpad) LoadShuttles() (int, error) {
	shuttles := []Shuttle{}

	file, err := os.Open(lp.ShuttlesPath)
	if err != nil {
		return 0, err
	}

	defer file.Close()

	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&shuttles); err != nil {
		return 0, err
	}

	for _, shuttle := range shuttles {
		lp.AddShuttle(shuttle)
	}

	return len(shuttles), nil
}

// Unexported since it relies on Launchpad.ShuttlesMutex being locked
func (lp *Launchpad) writeShuttles() error {
	file, err := os.Create(lp.ShuttlesPath)
	if err != nil {
		return err
	}

	defer file.Close()

	encoder := gob.NewEncoder(file)
	encoder.Encode(lp.Shuttles)

	return nil
}

func (lp *Launchpad) HasShuttle(shuttle Shuttle) bool {
	lp.ShuttlesMutex.Lock()
	defer lp.ShuttlesMutex.Unlock()

	found := false
	for _, otherShuttle := range lp.Shuttles {
		if shuttle.Route.Path == otherShuttle.Route.Path {
			found = true
			break
		}
	}

	return found
}

func (lp *Launchpad) AddShuttle(shuttle Shuttle) {
	if lp.HasShuttle(shuttle) {
		return
	}

	lp.ShuttlesMutex.Lock()
	defer lp.ShuttlesMutex.Unlock()

	lp.Shuttles = append(lp.Shuttles, shuttle)
	if err := lp.writeShuttles(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Failed to write shuttles, lets hope we don't crash...")
	}

	lp.Queue <- shuttle
}

func (lp *Launchpad) RemoveShuttle(shuttle Shuttle) {
	lp.ShuttlesMutex.Lock()
	defer lp.ShuttlesMutex.Unlock()

	shuttles := []Shuttle{}
	for _, otherShuttle := range lp.Shuttles {
		if shuttle.Route.Path != otherShuttle.Route.Path {
			shuttles = append(shuttles, otherShuttle)
		}
	}

	lp.Shuttles = shuttles
	if err := lp.writeShuttles(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Failed to write shuttles, lets hope we don't crash...")
	}
}

func (lp *Launchpad) LaunchShuttles() {
	for shuttle := range lp.Queue {
		if !lp.HasShuttle(shuttle) {
			// Discard the shuttle from queue
			continue
		}

		logger := log.WithFields(log.Fields{
			"path":     shuttle.Route.Path,
			"endpoint": shuttle.Route.Endpoint,
			"filename": shuttle.Filename,
		})

		logger.Info("Shuttle received, transporting to destination")

		_, statErr := os.Stat(shuttle.Path())
		if os.IsNotExist(statErr) {
			logger.Warning("Shuttle payload has gone missing, discarding")
			continue
		}

		lp.Enroute.Add(1)
		transportErr := shuttle.Route.Transport(shuttle.Filename)
		lp.Enroute.Done()

		if statErr != nil || transportErr != nil {
			logger.WithFields(log.Fields{
				"transportErr": transportErr,
				"statErr":      statErr,
			}).Error(fmt.Sprintf("Shuttle crashed, retrying in %d seconds", lp.Retry))

			go func(queue chan Shuttle, retry int, shuttle Shuttle) {
				time.Sleep(time.Duration(retry) * time.Second)
				queue <- shuttle
			}(lp.Queue, lp.Retry, shuttle)

			continue
		}

		lp.RemoveShuttle(shuttle)
		logger.Info("Shuttle arrived at the destination successfully")
	}
}
