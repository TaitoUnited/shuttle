package main

import (
	"flag"
	"io"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

func main() {
	var base, shuttlesPath string
	var retry, workers int

	start := time.Now()

	flag.StringVar(&base, "base", "REQUIRED", "Base path, the files are expected to be in [base]/[user]/files/")
	flag.StringVar(&shuttlesPath, "shuttles", "/run/shuttle/shuttles.gob", "Path to the file that contains persisted shuttles")
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

	missionControl := NewMissionControl(retry, shuttlesPath)
	if err := missionControl.Reload(base); err != nil {
		logger.WithFields(log.Fields{
			"err": err,
		}).Fatal("Failed to start up launchpad")
	}

	// Launch N threads that handle the uploads
	for i := 0; i < workers; i++ {
		go missionControl.Launchpad.LaunchShuttles()
	}

	logger.WithFields(log.Fields{
		"startup": time.Since(start),
	}).Info("Ready and processing")

	// Process in the old shuttles after start up
	logger = logger.WithFields(log.Fields{
		"path": shuttlesPath,
	})

	logger.Info("Loading in old shuttles")

	count, err := missionControl.Launchpad.LoadShuttles()
	if err != nil && err != io.EOF {
		logger.WithFields(log.Fields{
			"err": err,
		}).Error("Failed to load old shuttles, continuing operation")
	} else {
		logger.WithFields(log.Fields{
			"count": count,
		}).Info("Loaded old shuttles")
	}

	// Handle a SIGHUP as reloading routes, gracefully handle SIGINT / SIGTERM
	signalChannel := make(chan os.Signal, 3)
	signal.Notify(signalChannel, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	for sig := range signalChannel {
		if sig == syscall.SIGHUP {
			logger.Info("Reloading routes")

			if err := missionControl.Reload(base); err != nil {
				logger.WithFields(log.Fields{
					"err": err,
				}).Fatal("Failed to reload routes")
			}

			logger.Info("Routes reloaded")
			continue
		}

		if sig == syscall.SIGINT || sig == syscall.SIGTERM {
			logger.Info("Shutdown request received, waiting for transfers to complete...")
			missionControl.Launchpad.Enroute.Wait()
			logger.Info("Transfers complete, shutting down")

			break
		}

		logger.WithFields(log.Fields{
			"signal": sig,
		}).Error("Caught unwanted signal")
	}
}
