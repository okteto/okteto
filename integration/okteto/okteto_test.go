//go:build integration
// +build integration

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

package okteto

import (
	"log"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/pkg/model"
)

var (
	kubectlBinary = "kubectl"
	appsSubdomain = ""
	token         = ""
)

func TestMain(m *testing.M) {
	if v := os.Getenv(model.OktetoAppsSubdomainEnvVar); v != "" {
		appsSubdomain = v
	}

	if runtime.GOOS == "windows" {
		kubectlBinary = "kubectl.exe"
	}
	if _, err := exec.LookPath(kubectlBinary); err != nil {
		log.Printf("kubectl is not in the path: %s", err)
		os.Exit(1)
	}

	token = integration.GetToken()

	exitCode := m.Run()

	os.Exit(exitCode)
}
