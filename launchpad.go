package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Launchpad struct {
	Queue         chan Shuttle
	Retry         int
	Shuttles      []Shuttle
	ShuttlesMutex *sync.Mutex
}

func NewLaunchpad(retry int) Launchpad {
	return Launchpad{
		Queue:         make(chan Shuttle, 100),
		Retry:         retry,
		Shuttles:      []Shuttle{},
		ShuttlesMutex: &sync.Mutex{},
	}
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

		transportErr := shuttle.Route.Transport(shuttle.Filename)
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
