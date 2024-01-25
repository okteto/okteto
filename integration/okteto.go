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

package integration

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

// GetOktetoPath returns the okteto path used to run tests
// set OKTETO_PATH to the bin you want to test otherwise it'll
// use the one you have in your path
func GetOktetoPath() (string, error) {
	oktetoPath, ok := os.LookupEnv(model.OktetoPathEnvVar)
	if !ok {
		oktetoPath = "/usr/local/bin/okteto"
	}

	log.Printf("using %s", oktetoPath)

	var err error
	oktetoPath, err = filepath.Abs(oktetoPath)
	if err != nil {
		return "", err
	}
	output, err := RunOktetoVersion(oktetoPath)
	if err != nil {
		return "", fmt.Errorf("okteto version failed: %s - %w", output, err)
	}
	log.Println(output)
	return oktetoPath, nil
}

// GetToken returns the token used to run tests
func GetToken() string {
	var token string
	if v := os.Getenv(model.OktetoTokenEnvVar); v != "" {
		token = v
	} else {
		token = okteto.GetContext().Token
	}
	return token
}

// RunOktetoVersion runs okteto version given an oktetoPath
func RunOktetoVersion(oktetoPath string) (string, error) {
	cmd := exec.Command(oktetoPath, "version")
	cmd.Env = os.Environ()

	o, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("okteto version failed: %s - %w", string(o), err)
	}
	return string(o), nil
}

// GetTestNamespace returns the name for a namespace
func GetTestNamespace(prefix, user string) string {
	os := runtime.GOOS
	if os == "windows" {
		os = "win"
	}
	namespace := fmt.Sprintf("%s-%s-%d-%s", prefix, os, time.Now().Unix(), user)
	return strings.ToLower(namespace)
}

// GetCurrentNamespace returns the current namespace of the kubeconfig path
func GetCurrentNamespace() string {
	return kubeconfig.CurrentNamespace(config.GetKubeconfigPath())
}

// GetContentFromURL returns the content of the url
func GetContentFromURL(url string, timeout time.Duration) string {
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	retry := 0
	for {
		retry++
		select {
		case <-to.C:
			log.Printf("endpoint %s didn't respond", url)
			return ""
		case <-ticker.C:
			r, err := http.Get(url)
			if err != nil {
				if retry%10 == 0 {
					log.Printf("called %s, got %s, retrying", url, err)
				}
				continue
			}

			defer r.Body.Close()
			if r.StatusCode != http.StatusOK {
				if retry%10 == 0 {
					log.Printf("called %s, got status %d, retrying", url, r.StatusCode)
				}
				continue
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("could not read body: %s", err)
				return ""
			}
			content := strings.TrimSpace(string(body))
			if content == "" {
				if retry%10 == 0 {
					log.Printf("called %s, got empty content", url)
				}
				continue
			}

			return content
		}
	}
}

// SkipIfWindows skips a tests if is on a windows environment
func SkipIfWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping testing in windows CI environment")
	}
}

// SkipIfNotOktetoCluster skips a tests if is not on an okteto cluster
func SkipIfNotOktetoCluster(t *testing.T) {
	if !okteto.GetContext().IsOkteto {
		t.Skip("Skipping because is not on an okteto cluster")
	}
}
