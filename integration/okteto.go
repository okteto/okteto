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

package integration

import (
	"fmt"
	"log"
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

	cmd := exec.Command(oktetoPath, "version")
	cmd.Env = os.Environ()

	o, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("okteto version failed: %s - %s", string(o), err)
	}

	log.Println(string(o))
	return oktetoPath, nil
}

// RunOktetoKubeconfig runs okteto kubeconfig command
func RunOktetoKubeconfig(oktetoPath string) error {
	args := []string{"kubeconfig"}
	cmd := exec.Command(oktetoPath, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", oktetoPath, strings.Join(args, " "), string(o))
	}
	return nil
}

// RunOktetoCreateNamespace runs okteto namespace create
func RunOktetoCreateNamespace(oktetoPath, namespace string) error {
	okteto.CurrentStore = nil
	log.Printf("creating namespace %s", namespace)
	args := []string{"namespace", "create", namespace, "-l", "debug"}
	cmd := exec.Command(oktetoPath, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", oktetoPath, strings.Join(args, " "), string(o))
	}

	log.Printf("create namespace output: \n%s\n", string(o))

	n := okteto.Context().Namespace
	if namespace != n {
		return fmt.Errorf("current namespace is %s, expected %s", n, namespace)
	}
	if err := RunOktetoKubeconfig(oktetoPath); err != nil {
		return err
	}
	return nil
}

// RunOktetoNamespace runs okteto namespace command
func RunOktetoNamespace(oktetoPath, namespace string) error {
	okteto.CurrentStore = nil
	log.Printf("changing to namespace %s", namespace)
	args := []string{"namespace", namespace, "-l", "debug"}
	cmd := exec.Command(oktetoPath, args...)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", oktetoPath, strings.Join(args, " "), string(o))
	}

	log.Printf("namespace output: \n%s\n", string(o))

	n := okteto.Context().Namespace
	if namespace != n {
		return fmt.Errorf("current namespace is %s, expected %s", n, namespace)
	}
	args = []string{"kubeconfig"}
	cmd = exec.Command(oktetoPath, args...)
	cmd.Env = os.Environ()
	o, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", oktetoPath, strings.Join(args, " "), string(o))
	}

	return nil
}

// RunOktetoDeleteNamespace runs okteto namespace delete
func RunOktetoDeleteNamespace(oktetoPath, namespace string) error {
	log.Printf("okteto delete namespace %s", namespace)
	deleteCMD := exec.Command(oktetoPath, "namespace", "delete", namespace)
	deleteCMD.Env = os.Environ()
	o, err := deleteCMD.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto delete namespace failed: %s - %s", string(o), err)
	}
	return nil
}

// GetTestNamespace returns the name for a namespace
func GetTestNamespace(prefix, user string) string {
	namespace := fmt.Sprintf("%s-%s-%d-%s", prefix, runtime.GOOS, time.Now().Unix(), user)
	return strings.ToLower(namespace)
}

// GetCurrentNamespace returns the current namespace of the kubeconfig path
func GetCurrentNamespace() string {
	return kubeconfig.CurrentNamespace(config.GetKubeconfigPath())
}

// SkipIfWindows skips a tests if is on a windows environment
func SkipIfWindows(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping testing in CI environment")
	}
}
