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

package snapshot

import (
	"context"

	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Snapshot creates a snapshot command for use in further operations
func Snapshot(ctx context.Context, k8sLogger *io.K8sLogger, ioCtrl *io.Controller) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage volume snapshots",
		Long:  "Manage volume snapshots for your development environment",
	}

	k8sClientProvider := okteto.NewK8sClientProviderWithLogger(k8sLogger)

	cmd.AddCommand(Upload(ctx, k8sClientProvider, ioCtrl))
	return cmd
}