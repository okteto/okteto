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

	"github.com/okteto/okteto/cmd/utils"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type installFlags struct {
	oktetoLicense string
	kubeContext   string
	kubePath      string
}

// NewInstallCommand creates a server
func NewInstallCommand(ctx context.Context) *cobra.Command {
	flags := &installFlags{}
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Installs Okteto in a server",
		Args:  utils.NoArgsAccepted("https://www.okteto.com/docs/reference/cli/#deploy-1"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return installOktetoChart(ctx, *flags)
		},
	}

	cmd.Flags().StringVarP(&flags.oktetoLicense, "license", "", "", "the okteto license to apply")
	cmd.Flags().StringVarP(&flags.kubeContext, "kube-context", "", "", "the kubernetes context to use")
	cmd.Flags().StringVarP(&flags.kubePath, "kubepath", "", "", "the kubeconfig path to use")
	return cmd
}

func installOktetoChart(ctx context.Context, flags installFlags) error {
	var err error
	if flags.oktetoLicense == "" {
		flags.oktetoLicense, err = utils.AsksQuestion("Please provide a valid Okteto license: ")
		if err != nil {
			return err
		}
	}

	oktetoLog.Info("adding okteto chart to helm")
	cmd := exec.CommandContext(ctx, "helm", "repo", "add", "okteto", "https://charts.okteto.com")
	if flags.kubePath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", flags.kubePath))
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to setup stdout for session: %w", err)
	}

	go func() {
		if _, err := io.Copy(os.Stdout, stdout); err != nil {
			oktetoLog.Infof("error while writing to stdOut: %s", err)
		}
	}()
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("unable to setup stdout for session: %w", err)
	}

	go func() {
		if _, err := io.Copy(os.Stdout, stderr); err != nil {
			oktetoLog.Infof("error while writing to stdOut: %s", err)
		}
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	oktetoLog.Info("running helm repo update")
	cmd = exec.CommandContext(ctx, "helm", "repo", "update")
	if flags.kubePath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", flags.kubePath))
	}
	stdout, err = cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to setup stdout for session: %w", err)
	}

	go func() {
		if _, err := io.Copy(os.Stdout, stdout); err != nil {
			oktetoLog.Infof("error while writing to stdOut: %s", err)
		}
	}()
	stderr, err = cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("unable to setup stdout for session: %w", err)
	}

	go func() {
		if _, err := io.Copy(os.Stdout, stderr); err != nil {
			oktetoLog.Infof("error while writing to stdOut: %s", err)
		}
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	fs := afero.NewOsFs()
	configYAML, err := afero.TempFile(fs, "", "")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configYAML.Name(), []byte(fmt.Sprintf("license: %s", flags.oktetoLicense)), 0600); err != nil {
		return err
	}

	oktetoLog.Infof("Config file created at %s", configYAML.Name())

	oktetoLog.Info("installing okteto chart")
	cmd = exec.CommandContext(ctx, "helm", "upgrade", "--install", "okteto", "okteto/okteto", "-f", configYAML.Name(), "--namespace=okteto", "--create-namespace", "--debug")
	if flags.kubeContext != "" {
		cmd.Args = append(cmd.Args, "--kube-context", flags.kubeContext)
	}
	if flags.kubePath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", flags.kubePath))
	}
	stdout, err = cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to setup stdout for session: %w", err)
	}

	go func() {
		if _, err := io.Copy(os.Stdout, stdout); err != nil {
			oktetoLog.Infof("error while writing to stdOut: %s", err)
		}
	}()
	stderr, err = cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("unable to setup stdout for session: %w", err)
	}

	go func() {
		if _, err := io.Copy(os.Stdout, stderr); err != nil {
			oktetoLog.Infof("error while writing to stdOut: %s", err)
		}
	}()

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
