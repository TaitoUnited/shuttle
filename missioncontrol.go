package main

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

type MissionControl struct {
	Configuration Configuration
	Launchpad     Launchpad
	Services      []Service
}

func NewMissionControl(retry int, shuttlesPath string) MissionControl {
	launchpad := NewLaunchpad(retry, shuttlesPath)

	return MissionControl{
		Launchpad: launchpad,
	}
}

func (mc *MissionControl) Start() error {
	if err := mc.createDirectories(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Failed to create directories")

		return err
	}

	localRoutes, externalRoutes := SeparateRoutes(mc.Configuration.Routes)

	// SFTP
	sftp := NewSftpService(mc.Configuration.SftpHost, mc.Configuration.SftpPort, mc.Configuration.Base, externalRoutes, mc.Configuration.PrivateKey)
	mc.Services = append(mc.Services, sftp)

	// FTP
	ftp := NewFtpService(mc.Configuration.FtpHost, mc.Configuration.FtpPort, mc.Configuration.Base, mc.Configuration.Certificate, externalRoutes)
	mc.Services = append(mc.Services, ftp)

	// Web
	web := NewWebService(mc.Configuration.WebHost, mc.Configuration.WebPort, mc.Configuration.WebInsecurePort, mc.Configuration.WebAllowInsecure, mc.Configuration.Base, mc.Configuration.Certificate, externalRoutes)
	mc.Services = append(mc.Services, web)

	// Local
	local := NewLocalService(mc.Configuration.Base, localRoutes)
	mc.Services = append(mc.Services, local)

	// Start up everything
	for _, service := range mc.Services {
		if err := service.Start(); err != nil {
			log.WithFields(log.Fields{
				"err":     err,
				"service": service.Name(),
			}).Error("Failed to start service")

			return err
		}

		go mc.WatchWriteNotifications(service.WriteNotifications())

		log.WithFields(log.Fields{
			"service": service.Name(),
		}).Info("Service started")
	}

	return nil
}

func (mc *MissionControl) Stop() {
	log.Info("Waiting for services to shutdown...")

	for _, service := range mc.Services {
		if err := service.Stop(); err != nil {
			log.WithFields(log.Fields{
				"service": service.Name(),
				"err":     err,
			}).Warning("Failed to stop service gracefully")
		}
	}

	log.Info("Waiting for enroute shuttles to reach their destination...")

	mc.Launchpad.Enroute.Wait()
}

func (mc *MissionControl) WatchWriteNotifications(writeNotifications chan WriteNotification) {
	for writeNotification := range writeNotifications {
		shuttle, err := NewShuttleFromUsername(writeNotification.Path, writeNotification.Username, mc.Configuration.Routes)
		if err != nil {
			log.WithFields(log.Fields{
				"username": writeNotification.Username,
				"path":     writeNotification.Path,
				"err":      err,
			}).Error("Failed to create shuttle from write notification")
			continue
		}

		mc.Launchpad.AddShuttle(shuttle)
	}
}

func (mc *MissionControl) Reload(path string, ftpHost string, ftpPort int, sftpHost string, sftpPort int, webHost string, webPort int, webInsecurePort int, webAllowInsecure bool) error {
	configuration, err := NewConfiguration(path, ftpHost, ftpPort, sftpHost, sftpPort, webHost, webPort, webInsecurePort, webAllowInsecure)
	if err != nil {
		return err
	}

	mc.Configuration = configuration

	if err := mc.createDirectories(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Failed to create directories")

		return err
	}

	localRoutes, externalRoutes := SeparateRoutes(mc.Configuration.Routes)
	for _, service := range mc.Services {
		if _, ok := service.(*LocalService); ok {
			service.Reload(localRoutes)
		} else {
			service.Reload(externalRoutes)
		}
	}

	return nil
}

func (mc *MissionControl) createDirectories() error {
	for _, route := range mc.Configuration.Routes {
		path := filepath.Join(mc.Configuration.Base, route.Username, "failed")
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}

	return nil
}
