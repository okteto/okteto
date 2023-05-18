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
	"testing"

	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_SleepPreview(t *testing.T) {
	ctx := context.Background()
	currentPreview := "current"
	usr := &types.User{
		Token: "test-token",
	}
	initPreviews := []types.Namespace{
		{
			ID: currentPreview,
		},
		{
			ID: "test-1",
		},
	}
	var tests = []struct {
		name string
		// toSleepPr the preview to sleep
		toSleepPr                       string
		initialNamespacesAtOktetoClient []types.Namespace
		fakeOkClient                    *client.FakeOktetoClient
		fakeK8sClient                   *fake.Clientset
		err                             error
	}{
		{
			name:                            "sleeps existing preview, the current one",
			toSleepPr:                       currentPreview,
			initialNamespacesAtOktetoClient: initPreviews,
			fakeOkClient: &client.FakeOktetoClient{
				Namespace:    client.NewFakeNamespaceClient(initPreviews, nil),
				Users:        client.NewFakeUsersClient(usr),
				StreamClient: client.NewFakeStreamClient(&client.FakeStreamResponse{}),
			},
			fakeK8sClient: fake.NewSimpleClientset(),
		},
		{
			name:                            "sleeps existing preview, not the current one",
			toSleepPr:                       "test-1",
			initialNamespacesAtOktetoClient: initPreviews,
			fakeOkClient: &client.FakeOktetoClient{
				Namespace:    client.NewFakeNamespaceClient(initPreviews, nil),
				Users:        client.NewFakeUsersClient(usr),
				StreamClient: client.NewFakeStreamClient(&client.FakeStreamResponse{}),
			},
			fakeK8sClient: fake.NewSimpleClientset(),
		},
		{
			name:                            "sleep non existing preview",
			toSleepPr:                       "test-non-existing",
			initialNamespacesAtOktetoClient: initPreviews,
			fakeOkClient: &client.FakeOktetoClient{
				Namespace:    client.NewFakeNamespaceClient(initPreviews, assert.AnError),
				Users:        client.NewFakeUsersClient(usr),
				StreamClient: client.NewFakeStreamClient(&client.FakeStreamResponse{}),
			},
			fakeK8sClient: fake.NewSimpleClientset(),
			err:           errFailedSleepPreview,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// init ctx current store with initial values
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test-context": {
						Name:      "test-context",
						Token:     "test-token",
						IsOkteto:  true,
						Namespace: currentPreview,
						UserID:    "1",
					},
				},
				CurrentContext: "test-context",
			}
			nsFakeCommand := namespace.NewFakeNamespaceCommand(tt.fakeOkClient, tt.fakeK8sClient, usr)
			err := nsFakeCommand.ExecuteSleepNamespace(ctx, tt.toSleepPr)
			if tt.err != nil {
				assert.ErrorIs(t, errFailedSleepPreview, tt.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
