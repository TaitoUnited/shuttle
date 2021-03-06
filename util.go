package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
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

func DetectContentType(reader io.Reader) (string, error) {
	buffer := make([]byte, 512)

	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	return http.DetectContentType(buffer[:n]), nil
}

func SeparateRoutes(routes []Route) (local []Route, external []Route) {
	for _, route := range routes {
		if route.Local {
			local = append(local, route)
		} else {
			external = append(external, route)
		}
	}

	return
}
