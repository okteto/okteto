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

package preview

import (
	"context"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

type previewAnalyticsTracker interface {
	TrackDeployPreviewTriggered(ctx context.Context, m analytics.DeployPreviewTriggeredMetadata)
	TrackWakeTriggered(ctx context.Context, m analytics.WakeTriggeredMetadata)
}

type Command struct {
	okClient         types.OktetoInterface
	analyticsTracker previewAnalyticsTracker
}

// NewCommand creates a namespace command for previews
func NewCommand(at previewAnalyticsTracker) (*Command, error) {
	c, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, err
	}
	return &Command{
		okClient:         c,
		analyticsTracker: at,
	}, nil
}

// Preview preview management commands
func Preview(ctx context.Context, at previewAnalyticsTracker) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Preview Environment management commands",
	}

	cmd.AddCommand(Deploy(ctx, at))
	cmd.AddCommand(Destroy(ctx))
	cmd.AddCommand(List(ctx))
	cmd.AddCommand(Endpoints(ctx))
	cmd.AddCommand(Sleep(ctx, at))
	cmd.AddCommand(Wake(ctx, at))
	return cmd
}
