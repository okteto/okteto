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

package namespace

import (
	"context"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_WakeNamespace(t *testing.T) {
	ctx := context.Background()
	currentNamespace := "current"
	usr := &types.User{
		Token: "test-token",
	}
	initNamespaces := []types.Namespace{
		{
			ID: currentNamespace,
		},
		{
			ID: "test-1",
		},
	}
	var tests = []struct {
		err           error
		fakeOkClient  *client.FakeOktetoClient
		fakeK8sClient *fake.Clientset
		name          string
		// toWakeNs the namespace to wake
		toWakeNs                        string
		initialNamespacesAtOktetoClient []types.Namespace
	}{
		{
			name:                            "wakes existing ns, the current one",
			toWakeNs:                        currentNamespace,
			initialNamespacesAtOktetoClient: initNamespaces,
			fakeOkClient: &client.FakeOktetoClient{
				Namespace:    client.NewFakeNamespaceClient(initNamespaces, nil),
				Users:        client.NewFakeUsersClient(usr),
				StreamClient: client.NewFakeStreamClient(&client.FakeStreamResponse{}),
			},
			fakeK8sClient: fake.NewSimpleClientset(),
		},
		{
			name:                            "wakes existing ns, not the current one",
			toWakeNs:                        "test-1",
			initialNamespacesAtOktetoClient: initNamespaces,
			fakeOkClient: &client.FakeOktetoClient{
				Namespace:    client.NewFakeNamespaceClient(initNamespaces, nil),
				Users:        client.NewFakeUsersClient(usr),
				StreamClient: client.NewFakeStreamClient(&client.FakeStreamResponse{}),
			},
			fakeK8sClient: fake.NewSimpleClientset(),
		},
		{
			name:                            "wakes a non existing ns",
			toWakeNs:                        "test-non-existing",
			initialNamespacesAtOktetoClient: initNamespaces,
			fakeOkClient: &client.FakeOktetoClient{
				Namespace:    client.NewFakeNamespaceClient(initNamespaces, assert.AnError),
				Users:        client.NewFakeUsersClient(usr),
				StreamClient: client.NewFakeStreamClient(&client.FakeStreamResponse{}),
			},
			fakeK8sClient: fake.NewSimpleClientset(),
			err:           errFailedWakeNamespace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// init ctx current store with initial values
			okteto.CurrentStore = &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"test-context": {
						Name:      "test-context",
						Token:     "test-token",
						IsOkteto:  true,
						Namespace: currentNamespace,
						UserID:    "1",
					},
				},
				CurrentContext: "test-context",
			}
			nsFakeCommand := NewFakeNamespaceCommand(tt.fakeOkClient, tt.fakeK8sClient, usr)
			err := nsFakeCommand.ExecuteWakeNamespace(ctx, tt.toWakeNs)
			if tt.err != nil {
				assert.ErrorIs(t, err, tt.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
