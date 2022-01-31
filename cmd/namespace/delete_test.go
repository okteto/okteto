// Copyright 2022 The Okteto Authors
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

func Test_deleteNamespace(t *testing.T) {
	ctx := context.Background()
	personalNs := "personal"
	currentNs := "test"
	var tests = []struct {
		name              string
		toDeleteNs        string
		currentNamespaces []types.Namespace
		finalNs           string
		err               bool
	}{
		{
			name:       "delete existing ns, the current one",
			toDeleteNs: currentNs,
			currentNamespaces: []types.Namespace{
				{
					ID: "test-1",
				},
				{
					ID: currentNs,
				},
				{
					ID: personalNs,
				},
			},
			finalNs: personalNs,
		},
		{
			name:       "delete existing ns but not the current one",
			toDeleteNs: "test-1",
			currentNamespaces: []types.Namespace{
				{
					ID: "test-1",
				},
				{
					ID: currentNs,
				},
				{
					ID: personalNs,
				},
			},
			finalNs: currentNs,
		},
		{
			name:       "delete non-existing ns",
			toDeleteNs: "test-1",
			currentNamespaces: []types.Namespace{
				{
					ID: "test",
				},
				{
					ID: currentNs,
				},
				{
					ID: personalNs,
				},
			},
			finalNs: currentNs,
			err:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Name:              "test",
						Token:             "test",
						PersonalNamespace: personalNs,
						IsOkteto:          true,
						Namespace:         currentNs,
						UserID:            "1",
					},
				},
				CurrentContext: "test",
			}
			usr := &types.User{
				Token: "test",
			}
			fakeOktetoClient := &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient(tt.currentNamespaces, nil),
				Preview:   client.NewFakePreviewClient(nil, nil),
				Users:     client.NewFakeUsersClient(usr, nil),
			}
			nsCmd := &NamespaceCommand{
				okClient: fakeOktetoClient,
				ctxCmd:   newFakeContextCommand(fakeOktetoClient, usr),
			}
			err := nsCmd.ExecuteDeleteNamespace(ctx, tt.toDeleteNs)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.finalNs, okteto.Context().Namespace)

			ns, err := fakeOktetoClient.Namespaces().List(ctx)
			assert.Equal(t, nil, err)
			for _, n := range ns {
				assert.NotEqual(t, n.ID, tt.toDeleteNs)
			}
		})
	}
}
