# shuttle
Shuttle is a service that takes in files via a variety of methods (SFTP, FTP / FTPS...) and uploads them via HTTP to configured endpoints.

## Starting up

Install Shuttle into your `GOPATH`:
```bash
$ go get github.com/TaitoUnited/shuttle
```

Run Shuttle:
```bash
$ $GOPATH/bin/shuttle -config /etc/shuttle/config.json
```

Example configuration file is included in this repository as `config.json`.

## Command line parameters

You can see all command line parameters by providing the `-help` flag as so:
```bash
$ $GOPATH/bin/shuttle -help
```

Here is the output:
```plaintext
Usage of shuttle:
  -config string
    	Path to the config file (default "/etc/shuttle/config.json")
  -ftp-host string
    	Host that the FTP service will listen on (default "0.0.0.0")
  -ftp-port int
    	Port that the FTP service will listen on (default 2001)
  -retry int
    	Delay before restarting error-inducing shuttles (default 5)
  -sftp-host string
    	Host that the SFTP service will listen on (default "0.0.0.0")
  -sftp-port int
    	Port that the SFTP service will listen on (default 2002)
  -shuttles string
    	Path to the file that contains persisted shuttles (default "/run/shuttle/shuttles.gob")
  -workers int
    	Concurrent uploads (default 5)
```

## Configuration

The configuration is a JSON file with the following fields:

* base
  * The base folder that all user folders should be within
* routes
  * A list of routes, i.e. users and endpoints
    * username
      * Username that is used to login, directory `$base/$username` should exist
    * password
      * Password for the user hashed using bcrypt
    * endpoint
      * URL of the endpoint where files should be pushed to
    * local
      * Whether this user should have access to FTP, SFTP etc. or if the user folder should be monitored for files
* private_key
  * SSH private key for the SFTP service
* certificate_public
  * TLS certificate for FTPS
* certificate_private
  * Private key for the certificate specified in `certificate_public`

## Structure

Shuttle consists of Services, for example SftpService and FtpService. A user can be either local or non-local.

SftpService and FtpService are non-local services that allow the user to upload files which are then pushed to the specified endpoint URL using HTTP POST multipart form with `payload` as the file key.

If a user is marked as local, they cannot login to any of the non-local services. However, a local service, LocalService, will be monitoring their user folder for newly created files that can be placed there by any means, for example by a legacy application.

A file transfer to the endpoint URL is retried as long as the server does not respond. If a server sends any reply, even if it's HTTP 500 Internal Server Error, the transfer is considered successful and the file is removed from the user folder.

When Shuttle is shutdown using SIGTERM it will gracefully wait for all file transfers (client -> Shuttle and Shuttle -> endpoint) to finish before shutting down. All file transfers from Shuttle to the endpoints that are in progress or waiting for retry are stored in a file. In case the application crashes or is killed, the transfers can be retried.

HTTP basic auth credentials can be passed in the URL in the standard `http://username:password@example.com` form. Query parameters are also supported since the URL is handled as is. No other credential passing (cookies, headers, etc.) are currently supported.
