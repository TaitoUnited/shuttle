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
	var retry, workers int

	flag.StringVar(&base, "base", "REQUIRED", "Base path, the files are expected to be in [base]/[user]/files/")
	flag.IntVar(&retry, "retry", 5, "Retry delay for error-inducing shuttles")
	flag.IntVar(&workers, "workers", 5, "Threads that are handling uploads, i.e. the amount of concurrent uploads")
	flag.Parse()

	if base == "REQUIRED" {
		panic("Missing -base argument")
	}

	// Normalize the path, will not end in a slash
	base = path.Clean(base)

	logger := log.WithFields(log.Fields{
		"base": base,
	})

	missionControl := NewMissionControl(retry)
	if err := missionControl.Reload(base); err != nil {
		logger.WithFields(log.Fields{
			"err": err,
		}).Fatal("Failed to start up launchpad")
	}

	// Launch N threads that handle the uploads
	for i := 0; i < workers; i++ {
		go missionControl.Launchpad.LaunchShuttles()
	}

	logger.Info("Ready and processing")

	// Handle a SIGHUP as reloading routes
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGHUP)
	for _ = range signalChannel {
		logger.Info("Reloading routes")

		if err := missionControl.Reload(base); err != nil {
			logger.WithFields(log.Fields{
				"err": err,
			}).Fatal("Failed to reload routes")
		}

		logger.Info("Routes reloaded")
	}
}
