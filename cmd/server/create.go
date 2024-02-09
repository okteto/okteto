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

package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

type createFlags struct {
	provider      string
	oktetoLicense string
}

// NewCreateCommand creates a server
func NewCreateCommand(ctx context.Context) *cobra.Command {
	flags := &createFlags{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a server",
		Args:  utils.NoArgsAccepted("https://www.okteto.com/docs/reference/cli/#deploy-1"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateProvider(flags.provider); err != nil {
				return err
			}
			okHome := config.GetOktetoHome()

			installFlags := &installFlags{
				oktetoLicense: flags.oktetoLicense,
			}

			if flags.provider == "gcp" {
				installFlags.kubePath = filepath.Join(okHome, "clusters", "hackatontest")
				if err := createGCPFiles(installFlags.kubePath); err != nil {
					return err
				}
			} else {
				installFlags.kubePath = filepath.Join(okHome, "clusters", "nevadito")
				if err := os.MkdirAll(installFlags.kubePath, 0700); err != nil {
					return err
				}
				if err := createMinikubeFiles(installFlags.kubePath); err != nil {
					return err
				}
				installFlags.kubeContext = "nevadito"
			}
			if err := create(installFlags.kubePath); err != nil {
				return err
			}

			return installOktetoChart(ctx, *installFlags)
		},
	}

	cmd.Flags().StringVarP(&flags.provider, "provider", "p", "", "procider to install the cluster (gcp, minikube)")
	cmd.Flags().StringVarP(&flags.oktetoLicense, "license", "", "", "the okteto license to apply")
	return cmd
}

func validateProvider(provider string) error {
	if provider != "gcp" && provider != "minikube" {
		return fmt.Errorf("provider %s is not valid", provider)
	}
	return nil
}

func createMinikubeFiles(tfFolder string) error {
	err := os.WriteFile(filepath.Join(tfFolder, "minikube.tf"), []byte(minikubeTF), 0644)
	if err != nil {
		return fmt.Errorf("failed to create minikube.tf file: %w", err)
	}
	oktetoLog.Infof("%s created successfully", filepath.Join(tfFolder, "minikube.tf"))
	return nil
}

func createGCPFiles(tfFolder string) error {
	err := os.WriteFile(filepath.Join(tfFolder, "gcp.tf"), []byte(gcpTF), 0644)
	if err != nil {
		return fmt.Errorf("failed to create minikube.tf file: %w", err)
	}
	oktetoLog.Infof("%s created successfully", filepath.Join(tfFolder, "gcp.tf"))
	return nil
}

func create(tfFolder string) error {
	oktetoLog.Infof("Running terraform init")
	tfInitCMD := exec.Command("terraform", "init")
	tfInitCMD.Dir = tfFolder
	tfInitCMD.Env = append(tfInitCMD.Env, fmt.Sprintf("KUBECONFIG=%s", tfFolder))
	stdout, err := tfInitCMD.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to setup stdout for session: %w", err)
	}

	go func() {
		if _, err := io.Copy(os.Stdout, stdout); err != nil {
			oktetoLog.Infof("error while writing to stdOut: %s", err)
		}
	}()
	stderr, err := tfInitCMD.StderrPipe()
	if err != nil {
		return fmt.Errorf("unable to setup stdout for session: %w", err)
	}

	go func() {
		if _, err := io.Copy(os.Stdout, stderr); err != nil {
			oktetoLog.Infof("error while writing to stdOut: %s", err)
		}
	}()

	if err := tfInitCMD.Run(); err != nil {
		return err
	}

	oktetoLog.Infof("Running terraform apply")
	tfApplyCMD := exec.Command("terraform", "apply", "-auto-approve")
	tfApplyCMD.Dir = tfFolder
	tfApplyCMD.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", filepath.Join(tfFolder, "config")))
	stdout, err = tfApplyCMD.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to setup stdout for session: %w", err)
	}

	go func() {
		if _, err := io.Copy(os.Stdout, stdout); err != nil {
			oktetoLog.Infof("error while writing to stdOut: %s", err)
		}
	}()

	stderr, err = tfApplyCMD.StderrPipe()
	if err != nil {
		return fmt.Errorf("unable to setup stdout for session: %w", err)
	}

	go func() {
		if _, err := io.Copy(os.Stdout, stderr); err != nil {
			oktetoLog.Infof("error while writing to stdOut: %s", err)
		}
	}()

	if err := tfApplyCMD.Run(); err != nil {
		return err
	}

	return nil
}
