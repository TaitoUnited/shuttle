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
	FtpHost       string `json:"ftp_host"`
	FtpPort       int    `json:"ftp_port"`
	SftpHost      string `json:"sftp_host"`
	SftpPort      int    `json:"sftp_port"`
}

// NewConfiguration returns a new configuration struct.
func NewConfiguration(path string) (Configuration, error) {
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
	return configuration, nil
}
