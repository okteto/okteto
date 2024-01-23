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

package commands

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

// NamespaceOptions defines the options that can be added to a build command
type NamespaceOptions struct {
	Namespace  string
	OktetoHome string
	Token      string
}

// RunOktetoCreateNamespace runs okteto namespace create
func RunOktetoCreateNamespace(oktetoPath string, namespaceOpts *NamespaceOptions) error {
	okteto.CurrentStore = nil
	log.Printf("creating namespace %s", namespaceOpts.Namespace)
	args := []string{"namespace", "create", namespaceOpts.Namespace, "-l", "debug"}
	cmd := exec.Command(oktetoPath, args...)
	cmd.Env = os.Environ()
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if namespaceOpts.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, namespaceOpts.Token))
	}
	if namespaceOpts.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, namespaceOpts.OktetoHome))
	}

	log.Printf("Running: %s\n", cmd.String())
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", oktetoPath, strings.Join(args, " "), string(o))
	}

	log.Printf("create namespace output: \n%s\n", string(o))

	return nil
}

// RunOktetoNamespace runs okteto namespace command
func RunOktetoNamespace(oktetoPath string, namespaceOpts *NamespaceOptions) error {
	okteto.CurrentStore = nil
	log.Printf("changing to namespace %s", namespaceOpts.Namespace)
	args := []string{"namespace", namespaceOpts.Namespace, "-l", "debug"}
	cmd := exec.Command(oktetoPath, args...)

	cmd.Env = os.Environ()
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if namespaceOpts.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, namespaceOpts.OktetoHome))
	}
	if namespaceOpts.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, namespaceOpts.Token))
	}
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", oktetoPath, strings.Join(args, " "), string(o))
	}

	log.Printf("namespace output: \n%s\n", string(o))

	n := okteto.GetContext().Namespace
	if namespaceOpts.Namespace != n {
		return fmt.Errorf("current namespace is %s, expected %s", n, namespaceOpts.Namespace)
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
func RunOktetoDeleteNamespace(oktetoPath string, namespaceOpts *NamespaceOptions) error {
	log.Printf("okteto delete namespace %s", namespaceOpts.Namespace)
	deleteCMD := exec.Command(oktetoPath, "namespace", "delete", namespaceOpts.Namespace)

	deleteCMD.Env = os.Environ()
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		deleteCMD.Env = append(deleteCMD.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}
	if namespaceOpts.Token != "" {
		deleteCMD.Env = append(deleteCMD.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, namespaceOpts.Token))
	}

	if namespaceOpts.OktetoHome != "" {
		deleteCMD.Env = append(deleteCMD.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, namespaceOpts.OktetoHome))
	}
	o, err := deleteCMD.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto delete namespace failed: %s - %w", string(o), err)
	}
	return nil
}
