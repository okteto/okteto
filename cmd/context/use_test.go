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

package context

import (
	"context"
	"os"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test_setSecrets(t *testing.T) {
	key := "key"
	expectedValue := "value"
	var tests = []struct {
		envs    map[string]string
		name    string
		secrets []env.Var
	}{
		{
			name: "create new env var from secret",
			secrets: []env.Var{
				{
					Name:  key,
					Value: expectedValue,
				},
			},
			envs: map[string]string{},
		},
		{
			name: "not overwrite env var from secret",
			secrets: []env.Var{
				{
					Name:  key,
					Value: "random-value",
				},
			},
			envs: map[string]string{
				key: expectedValue,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}
			exportPlatformVariablesToEnv(tt.secrets)
			assert.Equal(t, expectedValue, os.Getenv(key))
		})
	}
}

func TestUseContext_Force(t *testing.T) {
	ctx := context.Background()
	user := &types.User{
		Token: "test-token",
	}

	tests := []struct {
		name             string
		ctxStore         *okteto.ContextStore
		ctxOptions       *Options
		fakeOktetoClient *client.FakeOktetoClient
		fakeObjects      []runtime.Object
		expectedErr      bool
		validateFunc     func(t *testing.T, ctxStore *okteto.ContextStore, ctxOptions *Options)
	}{
		{
			name: "force flag deletes existing context and forces re-login",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"https://okteto.example.com": {
						Name:      "https://okteto.example.com",
						Token:     "existing-token",
						Namespace: "existing-namespace",
						IsOkteto:  true,
					},
				},
				CurrentContext: "https://okteto.example.com",
			},
			ctxOptions: &Options{
				Context:      "https://okteto.example.com",
				Force:        true,
				IsOkteto:     true,
				Save:         true,
				IsCtxCommand: true,
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: false,
			validateFunc: func(t *testing.T, ctxStore *okteto.ContextStore, ctxOptions *Options) {
				// Verify that the context was recreated (should exist after UseContext)
				assert.Contains(t, ctxStore.Contexts, "https://okteto.example.com")
				// Verify that the token was cleared during force operation
				assert.Empty(t, ctxOptions.Token)
				assert.False(t, ctxOptions.InferredToken)
			},
		},
		{
			name: "force flag with context without schema",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"okteto.example.com": {
						Name:      "okteto.example.com",
						Token:     "existing-token",
						Namespace: "existing-namespace",
						IsOkteto:  true,
					},
				},
				CurrentContext: "okteto.example.com",
			},
			ctxOptions: &Options{
				Context:      "okteto.example.com",
				Force:        true,
				IsOkteto:     true,
				Save:         true,
				IsCtxCommand: true,
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: false,
			validateFunc: func(t *testing.T, ctxStore *okteto.ContextStore, ctxOptions *Options) {
				// Verify that the context was recreated
				assert.Contains(t, ctxStore.Contexts, "https://okteto.example.com")
				// Verify that the token was cleared during force operation
				assert.Empty(t, ctxOptions.Token)
				assert.False(t, ctxOptions.InferredToken)
			},
		},
		{
			name: "force flag clears current context when deleting it",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"https://okteto.example.com": {
						Name:      "https://okteto.example.com",
						Token:     "existing-token",
						Namespace: "existing-namespace",
						IsOkteto:  true,
					},
				},
				CurrentContext: "https://okteto.example.com",
			},
			ctxOptions: &Options{
				Context:      "https://okteto.example.com",
				Force:        true,
				IsOkteto:     true,
				Save:         true,
				IsCtxCommand: true,
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: false,
			validateFunc: func(t *testing.T, ctxStore *okteto.ContextStore, ctxOptions *Options) {
				// The current context should be set back to the context after UseContext completes
				assert.Equal(t, "https://okteto.example.com", ctxStore.CurrentContext)
			},
		},
		{
			name: "force flag with non-existent context",
			ctxStore: &okteto.ContextStore{
				Contexts:       map[string]*okteto.Context{},
				CurrentContext: "",
			},
			ctxOptions: &Options{
				Context:      "https://okteto.example.com",
				Force:        true,
				IsOkteto:     true,
				Save:         true,
				IsCtxCommand: true,
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: false,
			validateFunc: func(t *testing.T, ctxStore *okteto.ContextStore, ctxOptions *Options) {
				// Context should be created normally
				assert.Contains(t, ctxStore.Contexts, "https://okteto.example.com")
				assert.Equal(t, "https://okteto.example.com", ctxStore.CurrentContext)
			},
		},
		{
			name: "no force flag preserves existing token",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"https://okteto.example.com": {
						Name:      "https://okteto.example.com",
						Token:     "existing-token",
						Namespace: "existing-namespace",
						IsOkteto:  true,
					},
				},
				CurrentContext: "https://okteto.example.com",
			},
			ctxOptions: &Options{
				Context:      "https://okteto.example.com",
				Force:        false,
				IsOkteto:     true,
				Save:         true,
				IsCtxCommand: true,
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: false,
			validateFunc: func(t *testing.T, ctxStore *okteto.ContextStore, ctxOptions *Options) {
				// Token should be preserved from existing context
				assert.Equal(t, "existing-token", ctxOptions.Token)
				assert.True(t, ctxOptions.InferredToken)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the context store
			okteto.CurrentStore = tt.ctxStore

			// Create the command with fake dependencies
			cmd := &Command{
				K8sClientProvider:    test.NewFakeK8sProvider(tt.fakeObjects...),
				LoginController:      test.NewFakeLoginController(user, nil),
				OktetoClientProvider: client.NewFakeOktetoClientProvider(tt.fakeOktetoClient),
				OktetoContextWriter:  test.NewFakeOktetoContextWriter(),
				kubetokenController:  newStaticKubetokenController(),
			}

			// Execute the UseContext method
			err := cmd.UseContext(ctx, tt.ctxOptions)

			// Validate the result
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Run custom validation if provided
			if tt.validateFunc != nil {
				tt.validateFunc(t, tt.ctxStore, tt.ctxOptions)
			}
		})
	}
}

func TestUseCommand_ForceFlag(t *testing.T) {
	// Test that the force flag is properly added to the command
	cmd := Use()
	
	// Check that the force flag exists
	forceFlag := cmd.Flags().Lookup("force")
	require.NotNil(t, forceFlag, "force flag should be defined")
	assert.Equal(t, "false", forceFlag.DefValue, "force flag should default to false")
	assert.Equal(t, "delete the corresponding configuration in the okteto context and force the user to log in again", forceFlag.Usage)
}
