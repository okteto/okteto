// Copyright 2025 The Okteto Authors
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

package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAdminCatalogURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "https URL is preserved",
			input: "https://cloud.okteto.com",
			want:  "https://cloud.okteto.com/admin/catalog/new",
		},
		{
			name:  "trailing slash stripped",
			input: "https://cloud.okteto.com/",
			want:  "https://cloud.okteto.com/admin/catalog/new",
		},
		{
			name:  "bare host gets https scheme",
			input: "cloud.okteto.com",
			want:  "https://cloud.okteto.com/admin/catalog/new",
		},
		{
			name:  "http scheme preserved",
			input: "http://localhost:8080",
			want:  "http://localhost:8080/admin/catalog/new",
		},
		{
			name:  "bare host with port",
			input: "localhost:8080",
			want:  "https://localhost:8080/admin/catalog/new",
		},
		{
			name:  "bare host with subpath",
			input: "example.com/okteto",
			want:  "https://example.com/okteto/admin/catalog/new",
		},
		{
			name:  "existing query and fragment are dropped",
			input: "https://cloud.okteto.com/?foo=bar#baz",
			want:  "https://cloud.okteto.com/admin/catalog/new",
		},
		{
			name:    "empty URL rejected",
			input:   "",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildAdminCatalogURL(tc.input)
			assert.Equal(t, tc.wantErr, err != nil)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRunAdd_InvokesOpenerWithURL(t *testing.T) {
	var captured string
	opener := func(target string) error {
		captured = target
		return nil
	}
	err := runAdd("https://cloud.okteto.com", opener)
	require.NoError(t, err)
	assert.Equal(t, "https://cloud.okteto.com/admin/catalog/new", captured)
}

func TestRunAdd_BrowserFailureDoesNotError(t *testing.T) {
	opener := func(string) error { return errors.New("no browser") }
	// We do not want CLI failure to block the user - they just get the URL printed.
	assert.NoError(t, runAdd("https://cloud.okteto.com", opener))
}

func TestRunAdd_EmptyContextURLReturnsError(t *testing.T) {
	opener := func(string) error { return nil }
	err := runAdd("", opener)
	require.Error(t, err)
}

func TestEnsureAdmin_AllowsAdmin(t *testing.T) {
	user := &client.FakeUserClient{Admin: true}
	require.NoError(t, ensureAdmin(context.Background(), user))
}

func TestEnsureAdmin_RejectsNonAdmin(t *testing.T) {
	user := &client.FakeUserClient{Admin: false}
	err := ensureAdmin(context.Background(), user)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNotAdmin.E)
}

func TestEnsureAdmin_GenericAPIFailureFailsClosed(t *testing.T) {
	user := &client.FakeUserClient{}
	user.SetIsAdminErr(errors.New("network unreachable"))
	// Fail-closed on unknown errors so auth/config problems surface here rather
	// than confusing the user at the UI submission step.
	err := ensureAdmin(context.Background(), user)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not verify administrator privileges")
}

func TestEnsureAdmin_OldServerWithoutSuperFieldSoftFails(t *testing.T) {
	user := &client.FakeUserClient{}
	user.SetIsAdminErr(errors.New(`graphql: Cannot query field "super" on type "me"`))
	// Very old servers do not expose `super`; the UI still enforces admin
	// authorization on submission, so we proceed instead of blocking.
	assert.NoError(t, ensureAdmin(context.Background(), user))
}

func TestExecuteAdd_NonAdminDoesNotOpenBrowser(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts:       map[string]*okteto.Context{"test": {Name: "https://cloud.okteto.com"}},
	}
	cmd := &Command{
		okClient: &client.FakeOktetoClient{
			Users: &client.FakeUserClient{Admin: false},
		},
	}
	var opened bool
	opener := func(string) error {
		opened = true
		return nil
	}
	err := cmd.ExecuteAdd(context.Background(), opener)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNotAdmin.E)
	assert.False(t, opened, "browser must not open when the user is not an admin")
}

func TestExecuteAdd_AdminOpensBrowser(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts:       map[string]*okteto.Context{"test": {Name: "https://cloud.okteto.com"}},
	}
	cmd := &Command{
		okClient: &client.FakeOktetoClient{
			Users: &client.FakeUserClient{Admin: true},
		},
	}
	var captured string
	opener := func(target string) error {
		captured = target
		return nil
	}
	require.NoError(t, cmd.ExecuteAdd(context.Background(), opener))
	assert.Equal(t, "https://cloud.okteto.com/admin/catalog/new", captured)
}

// Compile-time assertion that FakeUserClient still satisfies UserInterface
// after the addition of IsAdmin, so this test file also guards the contract.
var _ types.UserInterface = (*client.FakeUserClient)(nil)
