package main

import (
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Shuttle struct {
	Path  string
	Route Route
}

func NewShuttle(path string, route Route) Shuttle {
	return Shuttle{
		Path:  path,
		Route: route,
	}
}

func NewShuttleFromUsername(path, username string, routes []Route) (Shuttle, error) {
	var shuttle Shuttle
	var route Route
	var found bool

	for _, otherRoute := range routes {
		if otherRoute.Username == username {
			found = true
			route = otherRoute
			break
		}
	}

	if !found {
		return shuttle, errors.New("Route missing")
	}

	shuttle = NewShuttle(path, route)
	return shuttle, nil
}

func (s Shuttle) Send() error {
	params := map[string]string{
		"username": s.Route.Username,
	}

	body, contentType, err := CreateMultipartForm(s.Path, params)
	if err != nil {
		return err
	}

	request, err := http.NewRequest("POST", s.Route.Endpoint, body)
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", contentType)

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	client := &http.Client{
		Transport: &http.Transport{
			Dial:                  dialer.Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}

	// This can fail but it's probably fine, no need to skip the rest
	io.Copy(ioutil.Discard, response.Body)
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		// Move the file to a failed folder
		if err := os.Rename(s.Path, filepath.Join(filepath.Dir(s.Path), "failed", filepath.Base(s.Path))); err != nil {
			return err
		}
	} else {
		// Remove the file
		if err := os.Remove(s.Path); err != nil {
			return err
		}
	}

	return nil
}
