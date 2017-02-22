package main

import (
	"encoding/json"
	"os"

	"golang.org/x/crypto/ssh"
)

// Configuration contains all the configuration variables.
// RawPrivateKey is never used except when populating PrivateKey,
// but it needs to be exported due to json.Decoder constraints.
type Configuration struct {
	Base          string  `json:"base"`
	Routes        []Route `json:"routes"`
	PrivateKey    ssh.Signer
	RawPrivateKey string `json:"private_key"`
	FtpHost       string
	FtpPort       int
	SftpHost      string
	SftpPort      int
}

// NewConfiguration returns a new configuration struct.
func NewConfiguration(path string, ftpHost string, ftpPort int, sftpHost string, sftpPort int) (Configuration, error) {
	var configuration Configuration

	handle, err := os.Open(path)
	if err != nil {
		return configuration, err
	}

	decoder := json.NewDecoder(handle)
	if err = decoder.Decode(&configuration); err != nil {
		return configuration, err
	}

	private, err := ssh.ParsePrivateKey([]byte(configuration.RawPrivateKey))
	if err != nil {
		return configuration, err
	}

	configuration.PrivateKey = private
	configuration.FtpHost = ftpHost
	configuration.FtpPort = ftpPort
	configuration.SftpHost = sftpHost
	configuration.SftpPort = sftpPort

	return configuration, nil
}