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
)

func Test_useNamespace(t *testing.T) {
	ctx := context.Background()
	currentNs := "test"
	var tests = []struct {
		name              string
		changeToNs        string
		currentNamespaces []types.Namespace
		expectedErr       bool
	}{
		{
			name: "Change to another ns",
			currentNamespaces: []types.Namespace{
				{
					ID: currentNs,
				},
				{
					ID: "test-1",
				},
			},
			changeToNs: "test-1",
		},
		{
			name: "Change to same ns",
			currentNamespaces: []types.Namespace{
				{
					ID: currentNs,
				},
			},
			changeToNs: currentNs,
		},
		{
			name: "Change to not existing ns",
			currentNamespaces: []types.Namespace{
				{
					ID: currentNs,
				},
			},
			changeToNs:  "test-1",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"test": {
						Name:     "test",
						Token:    "test",
						IsOkteto: true,
						UserID:   "1",
					},
				},
				CurrentContext: "test",
			}
			usr := &types.User{
				Token: "test",
			}
			fakeOktetoClient := &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient(tt.currentNamespaces, nil),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				Users:           client.NewFakeUsersClient(usr),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			}
			nsCmd := &Command{
				okClient: fakeOktetoClient,
				ctxCmd:   newFakeContextCommand(fakeOktetoClient, usr),
			}
			err := nsCmd.Use(ctx, tt.changeToNs)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				assert.Equal(t, tt.changeToNs, okteto.GetContext().Namespace)
			}

		})
	}
}
