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
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_deleteNamespace(t *testing.T) {
	ctx := context.Background()
	personalNamespace := "personal"
	currentNamespace := "current"
	usr := &types.User{
		Token: "test-token",
	}
	initNamespaces := []types.Namespace{
		{
			ID: currentNamespace,
		},
		{
			ID: personalNamespace,
		},
		{
			ID: "test-1",
		},
	}

	var tests = []struct {
		name string
		// toDeleteNs the namespace to delete
		toDeleteNs string
		// finalNs the namespace user should finally be
		finalNs                         string
		initialNamespacesAtOktetoClient []types.Namespace
		fakeOkClient                    *client.FakeOktetoClient
		fakeK8sClient                   *fake.Clientset
		err                             error
	}{
		{
			name:                            "delete existing ns, the current one",
			toDeleteNs:                      currentNamespace,
			initialNamespacesAtOktetoClient: initNamespaces,
			finalNs:                         personalNamespace,
			fakeOkClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient(initNamespaces, nil),
				Users:           client.NewFakeUsersClient(usr),
				StreamClient:    client.NewFakeStreamClient(&client.FakeStreamResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			fakeK8sClient: fake.NewSimpleClientset(),
		},
		{
			name:                            "delete existing ns, not the current one",
			toDeleteNs:                      "test-1",
			initialNamespacesAtOktetoClient: initNamespaces,
			finalNs:                         currentNamespace,
			fakeOkClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient(initNamespaces, nil),
				Users:           client.NewFakeUsersClient(usr),
				StreamClient:    client.NewFakeStreamClient(&client.FakeStreamResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			fakeK8sClient: fake.NewSimpleClientset(),
		},
		{
			name:                            "delete non-existing ns",
			toDeleteNs:                      "test-non-existing",
			initialNamespacesAtOktetoClient: initNamespaces,
			finalNs:                         currentNamespace,
			err:                             errFailedDeleteNamespace,
			fakeOkClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient(initNamespaces, nil),
				Users:           client.NewFakeUsersClient(usr),
				StreamClient:    client.NewFakeStreamClient(&client.FakeStreamResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			fakeK8sClient: fake.NewSimpleClientset(),
		},
		{
			name:                            "delete namespace failed at job",
			toDeleteNs:                      currentNamespace,
			initialNamespacesAtOktetoClient: initNamespaces,
			finalNs:                         currentNamespace,
			fakeOkClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient(initNamespaces, nil),
				Users:           client.NewFakeUsersClient(usr),
				StreamClient:    client.NewFakeStreamClient(&client.FakeStreamResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			fakeK8sClient: fake.NewSimpleClientset(&v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: currentNamespace,
					Labels: map[string]string{
						constants.NamespaceStatusLabel: "DeleteFailed",
					},
				},
			}),
			err: errFailedDeleteNamespace,
		},
		{
			name:                            "delete namespace stream logs failed",
			toDeleteNs:                      currentNamespace,
			initialNamespacesAtOktetoClient: initNamespaces,
			finalNs:                         personalNamespace,
			fakeOkClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient(initNamespaces, nil),
				Users:           client.NewFakeUsersClient(usr),
				StreamClient:    client.NewFakeStreamClient(&client.FakeStreamResponse{StreamErr: assert.AnError}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			fakeK8sClient: fake.NewSimpleClientset(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// init ctx current store with initial values
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test-context": {
						Name:              "test-context",
						Token:             "test-token",
						PersonalNamespace: personalNamespace,
						IsOkteto:          true,
						Namespace:         currentNamespace,
						UserID:            "1",
					},
				},
				CurrentContext: "test-context",
			}

			nsFakeCommand := NewFakeNamespaceCommand(tt.fakeOkClient, tt.fakeK8sClient, usr)
			err := nsFakeCommand.ExecuteDeleteNamespace(ctx, tt.toDeleteNs)
			assert.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.finalNs, okteto.Context().Namespace)

			// check namespace has been deleted from list
			ns, err := tt.fakeOkClient.Namespaces().List(ctx)
			// no error for this namespace list
			assert.Equal(t, nil, err)
			for _, n := range ns {
				assert.NotEqual(t, n.ID, tt.toDeleteNs)
			}
		})
	}
}
