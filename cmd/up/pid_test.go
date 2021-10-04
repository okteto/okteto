// Copyright 2021 The Okteto Authors
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
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/okteto/okteto/pkg/config"
)

func TestCreatePIDFile(t *testing.T) {
	deploymentName := "deployment"
	namespace := "namespace"
	if err := createPIDFile(namespace, deploymentName); err != nil {
		t.Fatal("unable to create pid file")
	}

	filePath := filepath.Join(config.GetAppHome(namespace, deploymentName), "okteto.pid")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("didn't create pid file")
	}

	filePID, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("pid file is corrupted")
	}
	if string(filePID) != strconv.Itoa(os.Getpid()) {
		t.Fatal("pid file content is invalid")
	}

	cleanPIDFile(namespace, deploymentName)
	if _, err := os.Create(filePath); os.IsExist(err) {
		t.Fatal("didn't delete pid file")
	}

}
