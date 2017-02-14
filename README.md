# shuttle
Shuttle is a service that takes in files via a variety of methods (SFTP, FTP / FTPS...) and uploads them via HTTP to configured endpoints.

## Folder structure

The folder structure for the uploaded files is built around how SFTP and other similar services handle chroots. The root folder of the chroot must be owned by `root` and not writable to the chrooted user.

Folder structure:
```plain
[base] / [user] /
                  .endpoint
                  files / file1
                          file2
                          ...
```

## Configuration

The application is configured via command-line arguments. All other configuration comes from the folder structure managed by Ansible.

```plain
Usage of shuttle:
  -base string
    	Base path, the files are expected to be in [base]/[user]/files/ (default "REQUIRED")
  -retry int
    	Retry delay for error-inducing shuttles (default 5)
  -shuttles string
    	Path to the file that contains persisted shuttles (default "/run/shuttle/shuttles.gob")
  -workers int
    	Threads that are handling uploads, i.e. the amount of concurrent uploads (default 5)
```

## Functionality

When starting, the application first iterates through all folders found in the directory pointed to by `[base]`. For each directory, it checks for a `.endpoint` named file and treats the trimmed content as the URL to upload files to. If no such file exists, the directory is not considered for the next part.

After finding all valid upload folders, the application adds an `inotify` watcher to each of them.

When an `IN_CLOSE_WRITE` event is fired in one of the upload folders, the file that triggered the event is uploaded to the appropriate endpoint as a multipart form with `payload` as the file key.

If an error occurs during the upload to the endpoint, the request is almost always retried. This makes the Shuttle service uploading at-least-once instead of at-most-once.

The `inotify` approach was chosen because of the `IN_CLOSE_WRITE` event that is available, this way the application does not need to do anything to make sure that the transfer by the user has been finished before uploading the file to the appropriate endpoint.

The `inotify` approach does not address the case when the application is not running and a file is uploaded by the user. However, all file uploads that have been noticed by the application are persisted to disk so that they can be handled properly even if the application dies.

If a file upload is completely missed by the application it will never be uploaded. These are the conditions in which the application would miss the upload and why each condition does not matter:

* The application crashes or application is updated.
  * systemd will restart the application, start up time is roughly 170 microseconds.
* The server is restarted / crashes.
  * No file uploads can be made while the server is down and when it comes back up, systemd will start the application with a start up time of roughly 170 microseconds.

Users and folders are managed via Ansible. When users change, Ansible will reload the application (SIGHUP) which will trigger a hot reload of the folder structure with ~70 microseconds of downtime.

The application catches SIGINT, SIGTERM and waits for uploads to endpoints to finish before terminating.

## Adding a new user

Add a new entry under the `shuttle_conf.users` section of the `ansible-playbooks / host_vars / shuttle` file in the `devops` repository and run the playbook.
