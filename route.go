package main

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync"

	log "github.com/sirupsen/logrus"
)

type Route struct {
	Path     string
	Endpoint string
}

func (r Route) Transport(wg *sync.WaitGroup) {
	defer wg.Done()

	logger := log.WithFields(log.Fields{
		"path":     r.Path,
		"endpoint": r.Endpoint,
	})

	files, err := ioutil.ReadDir(r.Path)
	if err != nil {
		logger.WithFields(log.Fields{
			"err": err,
		}).Error("Failed to list files")

		return
	}

	for _, file := range files {
		if file.IsDir() || file.Name() == ".endpoint" {
			continue
		}

		filepath := path.Join(r.Path, file.Name())

		logger = logger.WithFields(log.Fields{
			"file": file.Name(),
		})

		logger.Info("Found a new payload, shuttling")

		body, contentType, err := CreateMultipartForm(filepath)
		if err != nil {
			logger.WithFields(log.Fields{
				"err": err,
				"len": body.Len(),
			}).Error("Failed to create a multipart form")

			continue
		}

		request, err := http.NewRequest("POST", r.Endpoint, body)
		if err != nil {
			logger.WithFields(log.Fields{
				"err": err,
				"len": body.Len(),
			}).Error("Failed to create a HTTP request")

			continue
		}

		request.Header.Set("Content-Type", contentType)

		client := &http.Client{}
		response, err := client.Do(request)
		if err != nil {
			logger.WithFields(log.Fields{
				"err": err,
			}).Error("HTTP request failed")

			continue
		}

		if _, err := io.Copy(ioutil.Discard, response.Body); err != nil {
			// This is probably fine, no need to skip the rest of the loop
			logger.WithFields(log.Fields{
				"err": err,
			}).Error("Failed to read response body")
		}

		response.Body.Close()

		if response.StatusCode < 200 || response.StatusCode > 299 {
			logger.WithFields(log.Fields{
				"status": response.StatusCode,
			}).Error("Got a non-200 status code")

			continue
		}

		logger.Info("Shuttle arrived at the destination, removing payload")
		if err := os.Remove(filepath); err != nil {
			logger.WithFields(log.Fields{
				"err": err,
			}).Error("Failed to remove the file")

			continue
		}
	}
}
