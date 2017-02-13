package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"os"
	"path"
)

func CreateMultipartForm(filepath string) (*bytes.Buffer, string, error) {
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

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return body, writer.FormDataContentType(), nil
}
