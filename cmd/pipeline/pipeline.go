// Copyright 2021 The Okteto Authors
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
	"github.com/spf13/cobra"
)

//Pipeline pipeline management commands
func Pipeline(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Pipeline management commands",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli#pipeline"),
	}
	cmd.AddCommand(deploy(ctx))
	cmd.AddCommand(destroy(ctx))
	return cmd
}
