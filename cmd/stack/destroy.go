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

package stack

import (
	"context"
	"os"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// Destroy destroys a stack
func Destroy(ctx context.Context) *cobra.Command {
	var stackPath []string
	var name string
	var namespace string
	var rm bool
	cmd := &cobra.Command{
		Use:   "destroy <name>",
		Short: "Destroy a compose",
		Args:  utils.MaximumNArgsAccepted(1, "https://www.okteto.com/docs/reference/okteto-cli/"),
		RunE: func(cmd *cobra.Command, args []string) error {
			oktetoLog.Warning("'okteto stack destroy' is deprecated in favor of 'okteto destroy', and will be removed in a future version")
			if len(stackPath) == 1 {
				workdir := filesystem.GetWorkdirFromManifestPath(stackPath[0])
				if err := os.Chdir(workdir); err != nil {
					return err
				}
				stackPath[0] = filesystem.GetManifestPathFromWorkdir(stackPath[0], workdir)
			}
			s, err := contextCMD.LoadStackWithContext(ctx, name, namespace, stackPath, afero.NewOsFs())
			if err != nil {
				return err
			}

			to, err := model.GetTimeout()
			if err != nil {
				return err
			}

			err = stack.Destroy(ctx, s, rm, to)
			analytics.TrackDestroyStack(err == nil)
			if err == nil {
				oktetoLog.Success("Compose '%s' successfully destroyed", s.Name)
			}
			return err
		},
	}
	cmd.Flags().StringArrayVarP(&stackPath, "file", "f", []string{}, "path to the compose manifest file")
	cmd.Flags().StringVarP(&name, "name", "", "", "overwrites the compose name")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "overwrites the compose namespace where the compose is destroyed")
	cmd.Flags().BoolVarP(&rm, "volumes", "v", false, "remove persistent volumes")
	return cmd
}
