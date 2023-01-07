//go:build actions
// +build actions

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

package actions

import (
	"log"
	"os"
	"regexp"
	"runtime"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/pkg/model"
)

const (
	githubHTTPSURL = "https://github.com/"
	versionRegex   = `\d+\.\d+\.\d+`
)

var (
	oktetoVersion = ""
	user          = ""
	kubectlBinary = "kubectl"
	appsSubdomain = "cloud.okteto.net"
)

func TestMain(m *testing.M) {
	if u, ok := os.LookupEnv(model.OktetoUserEnvVar); !ok {
		log.Println("OKTETO_USER is not defined")
		os.Exit(1)
	} else {
		user = u
	}

	if v := os.Getenv(model.OktetoAppsSubdomainEnvVar); v != "" {
		appsSubdomain = v
	}

	if runtime.GOOS == "windows" {
		kubectlBinary = "kubectl.exe"
	}

	versionOutput, err := integration.RunOktetoVersion("okteto")
	if err != nil {
		log.Println("okteto binary not found")
		os.Exit(1)
	}
	r, _ := regexp.Compile(versionRegex)

	oktetoVersion = r.FindString(versionOutput)
	os.Exit(m.Run())
}
