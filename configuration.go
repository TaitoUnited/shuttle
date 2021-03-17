package main

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"os"

	"golang.org/x/crypto/ssh"
)

// Configuration contains all the configuration variables.
// RawPrivateKey is never used except when populating PrivateKey,
// but it needs to be exported due to json.Decoder constraints.
type Configuration struct {
	Base                     string  `json:"base"`
	Routes                   []Route `json:"routes"`
	PrivateKey               ssh.Signer
	RawPrivateKey            string `json:"private_key"`
	Certificate              tls.Certificate
	RawCertificatePrivateKey string `json:"certificate_private"`
	RawCertificatePublicKey  string `json:"certificate_public"`
	FtpHost                  string
	FtpPort                  int
	SftpHost                 string
	SftpPort                 int
	WebHost                  string
	WebPort                  int
	WebInsecurePort          int
	WebAllowInsecure         bool
}

// NewConfiguration returns a new configuration struct.
func NewConfiguration(path string, privateKeyPath string, certificatePublicPath string, certificatePrivatePath string, ftpHost string, ftpPort int, sftpHost string, sftpPort int, webHost string, webPort int, webInsecurePort int, webAllowInsecure bool) (Configuration, error) {
	var configuration Configuration

	handle, err := os.Open(path)
	if err != nil {
		return configuration, err
	}

	decoder := json.NewDecoder(handle)
	if err = decoder.Decode(&configuration); err != nil {
		return configuration, err
	}

	var private ssh.Signer
	if privateKeyPath != "" {
		rawPrivate, err := ioutil.ReadFile(privateKeyPath)
		if err != nil {
			return configuration, err
		}

		private, err = ssh.ParsePrivateKey(rawPrivate)
		if err != nil {
			return configuration, err
		}
	} else {
		private, err = ssh.ParsePrivateKey([]byte(configuration.RawPrivateKey))
		if err != nil {
			return configuration, err
		}
	}

	var certificate tls.Certificate
	if certificatePublicPath != "" && certificatePrivatePath != "" {
		certificate, err = tls.LoadX509KeyPair(certificatePublicPath, certificatePrivatePath)
		if err != nil {
			return configuration, err
		}
	} else {
		certificate, err = tls.X509KeyPair([]byte(configuration.RawCertificatePublicKey), []byte(configuration.RawCertificatePrivateKey))
		if err != nil {
			return configuration, err
		}
	}

	configuration.PrivateKey = private
	configuration.Certificate = certificate
	configuration.FtpHost = ftpHost
	configuration.FtpPort = ftpPort
	configuration.SftpHost = sftpHost
	configuration.SftpPort = sftpPort
	configuration.WebHost = webHost
	configuration.WebPort = webPort
	configuration.WebInsecurePort = webInsecurePort
	configuration.WebAllowInsecure = webAllowInsecure

	return configuration, nil
}
