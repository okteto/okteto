// Copyright 2020 The Okteto Authors
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
	"fmt"

	"github.com/spf13/cobra"
)

const (
	//HelmRepoURL link to okteto stack helm repo
	HelmRepoURL = "https://apps.okteto.com"
	//HelmRepoName repo name for local link to okteto stack helm repo
	HelmRepoName = "okteto-charts"
	//HelmChartName chart name for okteto stack
	HelmChartName = "stacks"
	//HelmChartVersion chart version for okteto stack chart
	HelmChartVersion = "0.1.0"
)

//Stack stack management commands
func Stack(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "stack",
		Short:  fmt.Sprintf("Stack management commands"),
		Hidden: true,
	}
	cmd.AddCommand(Deploy(ctx))
	cmd.AddCommand(Destroy(ctx))
	return cmd
}
