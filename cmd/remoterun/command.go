// Copyright 2024 The Okteto Authors
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

package remoterun

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// RemoteRun starts the remote run command. This is the command executed in the
// remote environment when okteto deploy is executed with the remote flag
func RemoteRun(ctx context.Context, k8sLogger *io.K8sLogger) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "remote-run",
		Short:        "Remote run management commands. These are the commands to be run remotely",
		Hidden:       true,
		SilenceUsage: true,
	}

	cmd.AddCommand(Deploy(ctx, k8sLogger))
	cmd.AddCommand(Destroy(ctx))
	cmd.AddCommand(Test(ctx))
	return cmd
}

// getDeployable get the deployable entity from the OKTETO_DEPLOYABLE environment variable
func getDeployable() (deployable.Entity, error) {
	encodedDeployable := os.Getenv(constants.OktetoDeployableEnvVar)

	if encodedDeployable == "" {
		return deployable.Entity{
			Commands: []model.DeployCommand{},
		}, nil
	}

	b, err := base64.StdEncoding.DecodeString(encodedDeployable)
	if err != nil {
		return deployable.Entity{}, fmt.Errorf("invalid base64 encoded deployable: %w", err)
	}

	entity := deployable.Entity{}
	if err := yaml.Unmarshal(b, &entity); err != nil {
		return deployable.Entity{}, fmt.Errorf("invalid deployable: %w", err)
	}

	return entity, nil
}
