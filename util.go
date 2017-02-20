package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"os"
	"path"
)

func CreateMultipartForm(filepath string, params map[string]string) (*bytes.Buffer, string, error) {
	handle, err := os.Open(filepath)
	if err != nil {
		return nil, "", err
	}

	defer handle.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("payload", path.Base(filepath))
	if err != nil {
		return nil, "", err
	}

	if _, err := io.Copy(part, handle); err != nil {
		return nil, "", err
	}

	for key, value := range params {
		if err := writer.WriteField(key, value); err != nil {
			return nil, "", err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return body, writer.FormDataContentType(), nil
}

func DiffRoutes(old []Route, new []Route) ([]Route, []Route) {
	added := []Route{}
	removed := []Route{}

	for _, oldRoute := range old {
		found := false
		for _, newRoute := range new {
			if oldRoute.Path == newRoute.Path {
				found = true
				break
			}
		}

		if !found {
			removed = append(removed, oldRoute)
		}
	}

	for _, newRoute := range new {
		found := false
		for _, oldRoute := range old {
			if oldRoute.Path == newRoute.Path {
				found = true
				break
			}
		}

		if !found {
			added = append(added, newRoute)
		}
	}

	return added, removed
}
