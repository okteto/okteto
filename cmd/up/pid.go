// Copyright 2022 The Okteto Authors
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

	"github.com/okteto/okteto/pkg/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/afero"
)

const oktetoPIDFilename = "okteto.pid"

// PIDController creates get and removes the info about the OktetoPID
type pidController struct {
	pidFilePath string
	filesystem  afero.Fs
	pidProvider pidProvider
}

type pidProvider interface {
	provide() int
}

type osPIDProvider struct{}

// newPIDController creates a PIDController using the user filesystem
func newPIDController(ns, dpName string) pidController {
	return pidController{
		pidFilePath: filepath.Join(config.GetAppHome(ns, dpName), oktetoPIDFilename),
		filesystem:  afero.NewOsFs(),
		pidProvider: osPIDProvider{},
	}
}

func (pc pidController) create() error {
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
	return nil
}

func (pc pidController) get() (string, error) {
	bytes, err := afero.ReadFile(pc.filesystem, pc.pidFilePath)
	if err != nil {
		return "", fmt.Errorf("could not read PID: %w", err)
	}
	return string(bytes), nil
}

func (pc pidController) delete() {
	pid := pc.pidProvider.provide()
	filePID, err := pc.get()
	if err != nil {
		oktetoLog.Infof("unable to delete PID file at %s", pc.pidFilePath)
	}

	if strconv.Itoa(pid) != filePID {
		oktetoLog.Infof("okteto process with PID '%s' has the ownership of the file.")
		oktetoLog.Info("skipping deletion of PID file")
		return
	}
	err = pc.filesystem.Remove(pc.pidFilePath)
	if err != nil && !os.IsNotExist(err) {
		oktetoLog.Infof("unable to delete PID file at %s", pc.pidFilePath)
	}
}

func (osPIDProvider) provide() int {
	return os.Getpid()
}
