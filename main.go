package main

import (
	"flag"
	"os"
	"os/signal"
	"path"
	"syscall"

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

	launchpad := NewLaunchpad()
	if err := launchpad.Reload(base); err != nil {
		logger.WithFields(log.Fields{
			"err": err,
		}).Fatal("Failed to start up launchpad")
	}

	// Launch N threads that handle the uploads
	for i := 0; i < 5; i++ {
		go launchpad.LaunchShuttles()
	}

	logger.Info("Ready and processing")

	// Handle a SIGHUP as reloading routes
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGHUP)
	for _ = range signalChannel {
		logger.Info("Reloading routes")

		if err := launchpad.Reload(base); err != nil {
			logger.WithFields(log.Fields{
				"err": err,
			}).Fatal("Failed to reload routes")
		}

		logger.Info("Routes reloaded")
	}
}
