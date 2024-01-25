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

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/spf13/cobra"
)

type analyticsTrackerInterface interface {
	TrackImageBuild(...*analytics.ImageBuildMetadata)
}

// Stack stack management commands
func Stack(ctx context.Context, at analyticsTrackerInterface, ioCtrl *io.Controller) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "stack",
		Short:  "Stack management commands",
		Args:   utils.NoArgsAccepted("https://www.okteto.com/docs/reference/cli/#deploy"),
		Hidden: true,
	}
	cmd.AddCommand(deploy(ctx, at, ioCtrl))
	cmd.AddCommand(Destroy(ctx))
	cmd.AddCommand(Endpoints(ctx))
	return cmd
}
