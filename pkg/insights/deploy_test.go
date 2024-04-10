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

package insights

import (
	"context"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestTrackDeploy(t *testing.T) {
	ctx := context.Background()

	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {
				Cfg: &api.Config{},
			},
		},
	}

	testCases := []struct {
		name           string
		cmap           []runtime.Object
		expectedLength int
	}{
		{
			name:           "cmap not found",
			expectedLength: 0,
		},
		{
			name: "cmap found without phase",
			cmap: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: v1.ObjectMeta{
						Name:      "okteto-git-test",
						Namespace: "test-namespace",
					},
				},
			},
			expectedLength: 0,
		},
		{
			name: "cmap found with a wrong phase json",
			cmap: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: v1.ObjectMeta{
						Name:      "okteto-git-test",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"phases": "wrong",
					},
				},
			},
			expectedLength: 0,
		},
		{
			name: "cmap ok",
			cmap: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: v1.ObjectMeta{
						Name:      "okteto-git-test",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"phases":     "[{\"name\":\"commands\",\"duration\":1.0},{\"name\":\"build\",\"duration\":2.0}]",
						"repository": "https://test-repo.com",
					},
				},
			},
			expectedLength: 1,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ip := &Publisher{
				ioCtrl:            *io.NewIOController(),
				k8sClientProvider: test.NewFakeK8sProvider(tc.cmap...),
			}
			c, _, err := ip.k8sClientProvider.Provide(&api.Config{})
			require.NoError(t, err)

			// Check that there are no events
			events, err := c.EventsV1().Events("test-namespace").List(ctx, v1.ListOptions{})
			require.NoError(t, err)
			require.Len(t, events.Items, 0)

			ip.TrackDeploy(ctx, "test", "test-namespace", true)

			// Check that there is the expected number of events
			events, err = c.EventsV1().Events("test-namespace").List(ctx, v1.ListOptions{})
			require.NoError(t, err)
			require.Len(t, events.Items, tc.expectedLength)

			// Check that the event has the expected data
			if tc.expectedLength == 1 {
				e := events.Items[0]
				require.Equal(t, "okteto_insights_deploy", e.Reason)
				require.Equal(t, "deploy", e.Action)
				require.Equal(t, "Normal", e.Type)
				require.Equal(t, "okteto_insights_deploy", e.Reason)
				require.Equal(t, `{"devenv_name":"test","repository":"https://test-repo.com","namespace":"test-namespace","schema_version":"1.0","phases":[{"name":"commands","duration":1},{"name":"build","duration":2}],"success":true}`, e.Note)
				require.Equal(t, "test-namespace", e.Namespace)
				require.Equal(t, "cli", e.ReportingController)
			}
		})
	}

}
