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
	"fmt"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_listNamespace(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		err               error
		name              string
		currentNamespaces []types.Namespace
	}{
		{
			name: "List all ns",
			currentNamespaces: []types.Namespace{
				{
					ID: "test",
				},
				{
					ID: "test-1",
				},
			},
		},
		{
			name:              "error retrieving ns",
			currentNamespaces: nil,
			err:               fmt.Errorf("error retrieving ns"),
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
				Namespace: client.NewFakeNamespaceClient(tt.currentNamespaces, tt.err),
				Users:     client.NewFakeUsersClient(usr),
			}
			nsCmd := &Command{
				okClient: fakeOktetoClient,
				ctxCmd:   newFakeContextCommand(fakeOktetoClient, usr),
			}
			err := nsCmd.executeListNamespaces(ctx)
			if tt.err != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

		})
	}
}
