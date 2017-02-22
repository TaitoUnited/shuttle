package main

type Service interface {
	// Name should return the name of the service.
	Name() string

	// Start should start the service.
	Start() error

	// Reload should reload the service using new routes
	Reload(routes []Route) error

	// Stop should gracefully stop the service.
	Stop() error

	// WriteNotifications should return a channel that file write notifications are sent on.
	WriteNotifications() chan WriteNotification
}

type WriteNotification struct {
	Username string
	Path     string
}
