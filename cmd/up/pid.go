// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package up

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/fsnotify/fsnotify"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/afero"
)

const oktetoPIDFilename = "okteto.pid"

// PIDController creates get and removes the info about the OktetoPID
type pidController struct {
	filesystem         afero.Fs
	pidProvider        pidProvider
	pidWatcherProvider pidWatcherProvider
	watcher            pidWatcher
	pidFilePath        string
}

type pidProvider interface {
	provide() int
}

type osPIDProvider struct{}

type pidWatcherProvider interface {
	provide() (pidWatcher, error)
}

type fsnotifyWatcherProvider struct{}

type pidWatcher interface {
	Add(name string) error
	Close() error
	GetEventChannel() chan fsnotify.Event
	GetErrorChannel() chan error
}

type fsnotifyWatcherWrapper struct {
	watcher *fsnotify.Watcher
}

// newPIDController creates a PIDController using the user filesystem
func newPIDController(ns, dpName string) pidController {
	return pidController{
		pidFilePath:        filepath.Join(config.GetAppHome(ns, dpName), oktetoPIDFilename),
		filesystem:         afero.NewOsFs(),
		pidProvider:        osPIDProvider{},
		pidWatcherProvider: fsnotifyWatcherProvider{},
	}
}

// create creates the PID file containing the okteto PID
func (pc *pidController) create() error {
	file, err := pc.filesystem.Create(pc.pidFilePath)
	if err != nil {
		return fmt.Errorf("unable to create PID file at %s", pc.pidFilePath)
	}

	defer func() {
		if err := file.Close(); err != nil {
			oktetoLog.Debugf("Error closing file %s: %s", pc.pidFilePath, err)
		}
	}()

	oktetoPID := strconv.Itoa(pc.pidProvider.provide())
	if _, err := file.WriteString(oktetoPID); err != nil {
		return fmt.Errorf("unable to write to PID file at %s", pc.pidFilePath)
	}

	// initialize watcher if it's not already initialized
	if pc.watcher == nil {
		pc.watcher, err = pc.pidWatcherProvider.provide()
		if err != nil {
			return fmt.Errorf("could not create watcher for file '%s': %w", pc.pidFilePath, err)
		}

		if err := pc.watcher.Add(pc.pidFilePath); err != nil {
			return fmt.Errorf("could not add '%s' to watcher: %w", pc.pidFilePath, err)
		}
	}

	return nil
}

// get reads the PID file containing the okteto PID and return it as string
func (pc pidController) get() (string, error) {
	bytes, err := afero.ReadFile(pc.filesystem, pc.pidFilePath)
	if err != nil {
		return "", fmt.Errorf("could not read PID: %w", err)
	}
	return string(bytes), nil
}

// delete removes the PID file containing the okteto PID
func (pc pidController) delete() {
	pid := pc.pidProvider.provide()
	filePID, err := pc.get()
	if err != nil {
		oktetoLog.Infof("unable to delete PID file at %s: %s", pc.pidFilePath, err)
	}

	if err := pc.watcher.Close(); err != nil {
		oktetoLog.Infof("could not close watcher: %s", err)
	}

	if strconv.Itoa(pid) != filePID {
		oktetoLog.Infof("okteto process with PID '%s' has the ownership of the file.", filePID)
		oktetoLog.Info("skipping deletion of PID file")
		return
	}
	err = pc.filesystem.Remove(pc.pidFilePath)
	if err != nil && !os.IsNotExist(err) {
		oktetoLog.Infof("unable to delete PID file at %s: %s", pc.pidFilePath, err)
	}
}

// notifyIfPIDFileChange returns a message in the channel whenever the content of the PID file has changed
func (pc pidController) notifyIfPIDFileChange(notifyCh chan error) {
	for {
		select {
		case e, ok := <-pc.watcher.GetEventChannel():
			if !ok {
				continue
			}
			switch {
			case e.Op == fsnotify.Remove || e.Op == fsnotify.Rename:
				// If the PID file has been removed or moved to another path
				// okteto will try to recreate it and if it fails to recreate
				// we will wait until the other okteto process notices that needs
				// to exit.
				oktetoLog.Infof("file %s has been moved or removed", pc.pidFilePath)
				if err := pc.create(); err != nil {
					oktetoLog.Infof("could not recreate PID file '%s': %s", pc.pidFilePath, err.Error())
					return
				}
				continue
			case e.Op == fsnotify.Create:
				// If file has been created we just need to log it and continue waiting
				// for any change in the content of the file
				oktetoLog.Infof("file %s has been recreated", pc.pidFilePath)
				continue
			case e.Op == fsnotify.Chmod:
				// If file permissions has been modified we need to log it and continue
				// waiting for any change in the content of the file
				oktetoLog.Infof("permissions from %s have been modified", pc.pidFilePath)
				continue
			case e.Op == fsnotify.Write:
				// If file content has changed, we should verify that the okteto process
				// is no longer the owner of the development container and should return
				// an error as soon as possible to exit with a proper error. If the content
				// is the same we should wait until the content is modified
				oktetoLog.Infof("file %s content has been modified", pc.pidFilePath)
				pid := pc.pidProvider.provide()
				filePID, err := pc.get()
				if err != nil {
					oktetoLog.Infof("unable to get PID file at %s", pc.pidFilePath)
					continue
				}
				if strconv.Itoa(pid) != filePID {
					notifyCh <- oktetoErrors.UserError{
						E:    fmt.Errorf("development container has been deactivated by another 'okteto up' command"),
						Hint: "Use 'okteto exec' to open another terminal to your development container",
					}
					return
				}
				continue
			}
		case err, ok := <-pc.watcher.GetErrorChannel():
			if !ok {
				continue
			}
			// If there is an error on the filewatcher we should log and wait
			// until the other okteto process notices that needs to exit.
			oktetoLog.Info(err)
			return
		}
	}
}

func (osPIDProvider) provide() int {
	return os.Getpid()
}

func (n fsnotifyWatcherWrapper) Add(name string) error {
	return n.watcher.Add(name)
}

func (n fsnotifyWatcherWrapper) Close() error {
	return n.watcher.Close()
}
func (n fsnotifyWatcherWrapper) GetEventChannel() chan fsnotify.Event {
	return n.watcher.Events
}

func (n fsnotifyWatcherWrapper) GetErrorChannel() chan error {
	return n.watcher.Errors
}

func (fsnotifyWatcherProvider) provide() (pidWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return fsnotifyWatcherWrapper{
		watcher: watcher,
	}, nil
}
