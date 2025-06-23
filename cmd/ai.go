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

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/okteto/okteto/cmd/up"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/insights"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const aiManifestTemplate = `dev:
  ai:
    image: anthropic/claude-dev:latest
    autocreate: true
    workdir: /workspace
    command: ["claude-code"]
    sync:
      - .:/workspace
    persistentVolume:
      enabled: true
      size: 10Gi
    environment:
      - WORKSPACE_DIR=/workspace
    forward:
      - 8080:8080
    resources:
      requests:
        memory: "2Gi"
        cpu: "1"
      limits:
        memory: "4Gi"
        cpu: "2"
`

// AI creates a new "okteto ai" command that spawns a Claude Code development environment
func AI(at *analytics.Tracker, insights *insights.Publisher, ioCtrl *io.Controller, k8sLogger *io.K8sLogger, fs afero.Fs) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "Activate an AI Development Environment with Claude Code",
		Long: `The 'okteto ai' command quickly spawns a development environment with Claude Code running in Kubernetes.
This creates a pod that syncs your local directory, sets up a remote persistent volume, and runs Claude Code as an agent inside the cluster.`,
		Example: `# Start an AI development environment
okteto ai

# Start with a specific namespace
okteto ai --namespace my-namespace`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAICommand(at, insights, ioCtrl, k8sLogger, fs)
		},
	}

	return cmd
}

func runAICommand(at *analytics.Tracker, insights *insights.Publisher, ioCtrl *io.Controller, k8sLogger *io.K8sLogger, fs afero.Fs) error {
	// Check if okteto.yml already exists
	if _, err := os.Stat("okteto.yml"); err == nil {
		return fmt.Errorf("okteto.yml already exists in current directory. Please remove it or run from a different directory")
	}

	// Create the AI manifest in the current directory
	if err := ioutil.WriteFile("okteto.yml", []byte(aiManifestTemplate), 0644); err != nil {
		return fmt.Errorf("failed to create AI manifest: %w", err)
	}

	ioCtrl.Logger().Success("Created okteto.yml with AI development environment configuration")
	ioCtrl.Logger().Info("Starting AI development environment...")

	// Create the up command with the AI manifest
	upCmd := up.Up(at, insights, ioCtrl, k8sLogger, fs)

	// Execute the up command with the "ai" dev service
	upCmd.SetArgs([]string{"ai"})

	return upCmd.Execute()
}
