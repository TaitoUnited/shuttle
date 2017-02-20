package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
)

type Route struct {
	Path     string
	Username string
	Endpoint string
}

func NewRoute(base string, username string, endpoint string) Route {
	return Route{
		Path:     path.Join(base, username, "files"),
		Username: username,
		Endpoint: endpoint,
	}
}

func (r Route) Transport(filename string) error {
	filepath := path.Join(r.Path, filename)

	params := map[string]string{
		"username": r.Username,
	}

	body, contentType, err := CreateMultipartForm(filepath, params)
	if err != nil {
		return err
	}

	request, err := http.NewRequest("POST", r.Endpoint, body)
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", contentType)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}

	// This can fail but it's probably fine, no need to skip the rest
	io.Copy(ioutil.Discard, response.Body)
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("Received non-200 status code: %d", response.StatusCode)
	}

	if err := os.Remove(filepath); err != nil {
		return err
	}

	return nil
}
