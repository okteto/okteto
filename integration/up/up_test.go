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

package up

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	ps "github.com/mitchellh/go-ps"
	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/model"
)

var (
	user            = ""
	token           = ""
	kubectlBinary   = "kubectl"
	appsSubdomain   = "cloud.okteto.net"
	ErrUpNotRunning = errors.New("Up command is no longer running")
)

const (
	timeout         = 300 * time.Second
	stignoreContent = `venv
.okteto
.kube`
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
	if _, err := exec.LookPath(kubectlBinary); err != nil {
		log.Printf("kubectl is not in the path: %s", err)
		os.Exit(1)
	}
	token = integration.GetToken()

	exitCode := m.Run()
	os.Exit(exitCode)
}

func writeFile(filepath, content string) error {
	if err := os.WriteFile(filepath, []byte(content), 0600); err != nil {
		return err
	}
	return nil
}

func checkStignoreIsOnRemote(namespace, svcName, manifestPath, oktetoPath, dir string) error {
	opts := &commands.ExecOptions{
		Namespace:    namespace,
		ManifestPath: manifestPath,
		Command:      []string{"sh", "-c", `cat .stignore | grep '(?d)venv'`},
		OktetoHome:   dir,
		Token:        token,
		Service:      svcName,
	}
	output, err := commands.RunExecCommand(oktetoPath, opts)
	if err != nil {
		return err
	}
	if !strings.Contains(output, "venv") {
		return fmt.Errorf("okteto exec wrong output: %s", output)
	}
	return nil
}

func killLocalSyncthing(upPid int) error {
	processes, err := ps.Processes()
	if err != nil {
		return fmt.Errorf("fail to list processes: %s", err.Error())
	}
	for _, p := range processes {
		if p.Executable() == "syncthing" {
			pr, err := os.FindProcess(p.Pid())
			if err != nil {
				log.Printf("fail to find process %d : %s", p.Pid(), err)
				continue
			}
			if upPid == p.PPid() {
				if err := pr.Kill(); err != nil {
					log.Printf("fail to kill process %d : %s", p.Pid(), err)
				}
			}
		}
	}
	return nil
}

func waitUntilUpdatedContent(url, expectedContent string, timeout time.Duration, errorChan chan error) error {
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	contentTimeout := 5 * time.Second
	retry := 0
	for {
		select {
		case <-errorChan:
			return fmt.Errorf("okteto up is no longer running")
		case <-to.C:
			return fmt.Errorf("%s without updating %s to %s", timeout.String(), url, expectedContent)
		case <-ticker.C:
			retry++
			content := integration.GetContentFromURL(url, contentTimeout)
			if content == "" {
				continue
			}
			if content != expectedContent {
				if retry%10 == 0 {
					log.Printf("expected updated content to be %s, got %s\n", expectedContent, content)
				}
				continue
			}
			return nil
		}
	}
}
