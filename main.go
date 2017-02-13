package main

import (
	"flag"
	"path"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

func main() {
	var base string
	flag.StringVar(&base, "base", "REQUIRED", "Base path, the files are expected to be in [base]/[user]/files/")
	flag.Parse()

	if base == "REQUIRED" {
		panic("Missing -base argument")
	}

	// Normalize the path, will not end in a slash
	base = path.Clean(base)

	logger := log.WithFields(log.Fields{
		"base": base,
	})

	logger.Info("Discovering shuttle routes")

	launchpad := Launchpad{}
	routes, err := launchpad.DiscoverRoutes(base)
	if err != nil {
		logger.WithFields(log.Fields{
			"err": err,
		}).Fatal("Failed to discover routes")
	}

	logger.WithFields(log.Fields{
		"len": len(routes),
	}).Info("Shuttle routes discovered, watching for payloads")

	for {
		wg := &sync.WaitGroup{}
		for _, route := range routes {
			wg.Add(1)
			go route.Transport(wg)
		}

		wg.Wait()
		time.Sleep(10 * time.Second)
	}
}
