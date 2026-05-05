// Copyright 2026 The Okteto Authors
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

package context

import (
	"context"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/require"
)

type mockGroupsIdentifier struct {
	called bool
}

func (m *mockGroupsIdentifier) IdentifyGroups() {
	m.called = true
}

func TestInitOktetoContext_CallsIdentifyGroups(t *testing.T) {
	ctx := context.Background()
	user := &types.User{Token: "test"}

	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: make(map[string]*okteto.Context),
	}

	mock := &mockGroupsIdentifier{}
	cmd := newFakeContextCommand(&client.FakeOktetoClient{
		Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
		Users:           client.NewFakeUsersClient(user),
		Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
		KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
	}, user, nil)
	cmd.analyticsIdentifier = mock

	err := cmd.UseContext(ctx, &Options{
		IsOkteto: true,
		Context:  "https://okteto.example.com",
		Token:    "test",
	})

	require.NoError(t, err)
	require.True(t, mock.called, "IdentifyGroups must be called after successful context initialization")
}
