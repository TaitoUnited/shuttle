package main

import (
	"flag"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

func main() {
	var configPath, shuttlesPath string
	var retry, workers int

	start := time.Now()

	flag.StringVar(&configPath, "config", "/etc/shuttle/config.json", "Path to the config file")
	flag.StringVar(&shuttlesPath, "shuttles", "/run/shuttle/shuttles.gob", "Path to the file that contains persisted shuttles")
	flag.IntVar(&retry, "retry", 5, "Delay before restarting error-inducing shuttles")
	flag.IntVar(&workers, "workers", 5, "Concurrent uploads")
	flag.Parse()

	logger := log.WithFields(log.Fields{
		"path": configPath,
	})

	missionControl := NewMissionControl(retry, shuttlesPath)
	if err := missionControl.Reload(configPath); err != nil {
		logger.WithFields(log.Fields{
			"err": err,
		}).Fatal("Failed to load configuration")
	}

	logger = log.WithFields(log.Fields{
		"base": missionControl.Configuration.Base,
	})

	if err := missionControl.Start(); err != nil {
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
			logger.Info("Reloading configuration")

			if err := missionControl.Reload(configPath); err != nil {
				logger.WithFields(log.Fields{
					"err": err,
				}).Error("Failed to reload configuration")
				continue
			}

			logger.Info("Configuration reloaded")
			continue
		}

		if sig == syscall.SIGINT || sig == syscall.SIGTERM {
			logger.Info("Shutdown request received, waiting for clean exit...")
			missionControl.Stop()
			logger.Info("Clean exit complete, shutting down")

			break
		}

		logger.WithFields(log.Fields{
			"signal": sig,
		}).Error("Caught unwanted signal")
	}
}
