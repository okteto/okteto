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

package commands

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/okteto/okteto/pkg/okteto"
)

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
	return RunOktetoKubeconfig(oktetoPath)
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
