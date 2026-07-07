// Copyright 2023-2025 The Okteto Authors
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
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWakePreviewTracker struct {
	wakeCalls []analytics.WakeTriggeredMetadata
}

func (*fakeWakePreviewTracker) TrackDeployPreviewTriggered(_ context.Context, _ analytics.DeployPreviewTriggeredMetadata) {
}

func (f *fakeWakePreviewTracker) TrackWakeTriggered(_ context.Context, m analytics.WakeTriggeredMetadata) {
	f.wakeCalls = append(f.wakeCalls, m)
}

func Test_ExecuteWakePreview(t *testing.T) {
	tests := []struct {
		wakeErr       error
		name          string
		expectTracked bool
	}{
		{
			name:          "wake succeeds, tracks is_preview true",
			wakeErr:       nil,
			expectTracked: true,
		},
		{
			name:          "wake fails, does not track",
			wakeErr:       assert.AnError,
			expectTracked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tracker := &fakeWakePreviewTracker{}
			prCmd := &Command{
				okClient: &client.FakeOktetoClient{
					Namespace: client.NewFakeNamespaceClient(nil, tt.wakeErr),
				},
				analyticsTracker: tracker,
			}

			err := prCmd.ExecuteWakePreview(ctx, "my-preview")

			if tt.wakeErr != nil {
				require.ErrorIs(t, err, errFailedWakePreview)
			} else {
				require.NoError(t, err)
			}

			if tt.expectTracked {
				require.Len(t, tracker.wakeCalls, 1)
				require.Equal(t, "my-preview", tracker.wakeCalls[0].Namespace)
				require.True(t, tracker.wakeCalls[0].IsPreview)
			} else {
				require.Empty(t, tracker.wakeCalls)
			}
		})
	}
}
