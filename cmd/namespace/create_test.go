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

func Test_createNamespace(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		name    string
		newNs   string
		members *[]string
	}{
		{
			name:    "create new ns",
			newNs:   "test-1",
			members: nil,
		},
		{
			name:    "create new ns that exists",
			newNs:   "test",
			members: nil,
		},
		{
			name:    "create new ns with members",
			newNs:   "test",
			members: &[]string{"test-user"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Name:  "test",
						Token: "test",
					},
				},
				CurrentContext: "test",
			}
			usr := &types.User{
				Token: "test",
			}
			fakeOktetoClient := &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{{ID: "test"}}, nil),
				Users:           client.NewFakeUsersClient(usr),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			}
			nsCmd := &NamespaceCommand{
				okClient: fakeOktetoClient,
				ctxCmd:   newFakeContextCommand(fakeOktetoClient, usr),
			}
			err := nsCmd.Create(ctx, &CreateOptions{
				Members:   tt.members,
				Namespace: tt.newNs,
			})
			assert.Equal(t, nil, err)
			assert.Equal(t, tt.newNs, okteto.Context().Namespace)
		})
	}
}
