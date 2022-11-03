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
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

type fakePIDProvider struct {
	pid int
}

func (fpp fakePIDProvider) provide() int {
	return fpp.pid
}

func TestPIDController(t *testing.T) {
	t.Parallel()
	deploymentName := "deployment"
	namespace := "namespace"
	fakePIDProvider := fakePIDProvider{
		pid: 5,
	}

	pidController := pidController{
		filesystem:  afero.NewMemMapFs(),
		pidFilePath: filepath.Clean(fmt.Sprintf("/%s/%s", deploymentName, namespace)),
		pidProvider: fakePIDProvider,
	}

	err := pidController.create()
	assert.NoError(t, err)

	if _, err := pidController.filesystem.Stat(pidController.pidFilePath); os.IsNotExist(err) {
		t.Fatal("didn't create pid file")
	}

	pid, err := pidController.get()

	assert.NoError(t, err)

	assert.Equal(t, strconv.Itoa(fakePIDProvider.pid), pid)

	pidController.delete()

	if _, err := pidController.filesystem.Stat(pidController.pidFilePath); !os.IsNotExist(err) {
		t.Fatal("didn't remove pid file")
	}

}

func TestTwoPIDControllerRaceConditionToRemoveFile(t *testing.T) {
	t.Parallel()
	deploymentName := "deployment"
	namespace := "namespace"
	fakePP := fakePIDProvider{
		pid: 5,
	}

	filesystem := afero.NewMemMapFs()
	pc := pidController{
		filesystem:  filesystem,
		pidFilePath: filepath.Clean(fmt.Sprintf("/%s/%s", deploymentName, namespace)),
		pidProvider: fakePP,
	}

	err := pc.create()
	assert.NoError(t, err)

	if _, err := pc.filesystem.Stat(pc.pidFilePath); os.IsNotExist(err) {
		t.Fatal("didn't create pid file")
	}

	pid, err := pc.get()

	assert.NoError(t, err)

	assert.Equal(t, strconv.Itoa(fakePP.pid), pid)

	// Simulate that another oktet up session is modifying the file
	fakePIDProviderWitAnotherPID := fakePIDProvider{
		pid: 10,
	}
	pidControllerWitAnotherPID := pidController{
		filesystem:  filesystem,
		pidFilePath: filepath.Clean(fmt.Sprintf("/%s/%s", deploymentName, namespace)),
		pidProvider: fakePIDProviderWitAnotherPID,
	}

	err = pidControllerWitAnotherPID.create()
	assert.NoError(t, err)

	if _, err := pidControllerWitAnotherPID.filesystem.Stat(pidControllerWitAnotherPID.pidFilePath); os.IsNotExist(err) {
		t.Fatal("didn't create pid file")
	}

	pid, err = pidControllerWitAnotherPID.get()

	assert.NoError(t, err)

	assert.Equal(t, strconv.Itoa(fakePIDProviderWitAnotherPID.pid), pid)

	// finish the first controller (finish the first okteto up while the second up keep running)
	pc.delete()
	if _, err := pc.filesystem.Stat(pc.pidFilePath); err != nil {
		t.Fatal("pid was deleted")
	}

	pid, err = pidControllerWitAnotherPID.get()

	assert.NoError(t, err)

	assert.Equal(t, strconv.Itoa(fakePIDProviderWitAnotherPID.pid), pid)
}
