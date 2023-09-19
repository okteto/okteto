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

package pipeline

import (
	"context"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

type PipelineDeployerInterface interface {
	ExecuteDeployPipeline(ctx context.Context, opts *DeployOptions) error
}
type PipelineInterface interface {
	PipelineDeployerInterface
}

// Command has all the pipeline subcommands
type Command struct {
	okClient          types.OktetoInterface
	k8sClientProvider okteto.K8sClientProvider
}

// NewCommand creates a namespace command to
func NewCommand() (*Command, error) {
	var okClient = &okteto.OktetoClient{}
	if okteto.IsOkteto() {
		c, err := okteto.NewOktetoClient()
		if err != nil {
			return nil, err
		}
		okClient = c
	}
	return &Command{
		okClient:          okClient,
		k8sClientProvider: okteto.NewK8sClientProvider(),
	}, nil
}

// Pipeline pipeline management commands
func Pipeline(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Pipeline management commands",
		Args:  utils.NoArgsAccepted("https://www.okteto.com/docs/reference/cli/#pipeline"),
	}
	cmd.AddCommand(deploy(ctx))
	cmd.AddCommand(destroy(ctx))
	cmd.AddCommand(list(ctx))
	return cmd
}
