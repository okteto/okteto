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

package login

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestController_AuthenticateToOktetoCluster(t *testing.T) {
	controller := NewLoginController()
	ctx := context.Background()

	tests := []struct {
		name      string
		token     string
		expectErr bool
	}{
		{
			name:      "with token",
			token:     "test-token",
			expectErr: false,
		},
		{
			name:      "without token",
			token:     "",
			expectErr: true, // Will fail because we can't actually start browser in test
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := controller.AuthenticateToOktetoCluster(ctx, "https://test.okteto.com", tt.token)
			
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, tt.token, user.Token)
			}
		})
	}
}

func TestController_AuthenticateToOktetoClusterWithOptions(t *testing.T) {
	controller := NewLoginController()
	ctx := context.Background()

	tests := []struct {
		name      string
		token     string
		noBrowser bool
		expectErr bool
	}{
		{
			name:      "with token and no-browser true",
			token:     "test-token",
			noBrowser: true,
			expectErr: false,
		},
		{
			name:      "with token and no-browser false",
			token:     "test-token",
			noBrowser: false,
			expectErr: false,
		},
		{
			name:      "without token and no-browser true",
			token:     "",
			noBrowser: true,
			expectErr: true, // Will fail because we can't actually start browser in test
		},
		{
			name:      "without token and no-browser false",
			token:     "",
			noBrowser: false,
			expectErr: true, // Will fail because we can't actually start browser in test
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := controller.AuthenticateToOktetoClusterWithOptions(ctx, "https://test.okteto.com", tt.token, tt.noBrowser)
			
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, tt.token, user.Token)
			}
		})
	}
}

func TestController_AuthenticateToOktetoClusterWithOptions_TokenProvided(t *testing.T) {
	controller := NewLoginController()
	ctx := context.Background()
	token := "test-token-123"

	// When token is provided, noBrowser flag should not matter
	user1, err1 := controller.AuthenticateToOktetoClusterWithOptions(ctx, "https://test.okteto.com", token, true)
	user2, err2 := controller.AuthenticateToOktetoClusterWithOptions(ctx, "https://test.okteto.com", token, false)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotNil(t, user1)
	assert.NotNil(t, user2)
	assert.Equal(t, token, user1.Token)
	assert.Equal(t, token, user2.Token)
}

func TestController_BackwardCompatibility(t *testing.T) {
	controller := NewLoginController()
	ctx := context.Background()
	token := "test-token"

	// Test that the old method still works and behaves the same as the new method with noBrowser=false
	user1, err1 := controller.AuthenticateToOktetoCluster(ctx, "https://test.okteto.com", token)
	user2, err2 := controller.AuthenticateToOktetoClusterWithOptions(ctx, "https://test.okteto.com", token, false)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotNil(t, user1)
	assert.NotNil(t, user2)
	assert.Equal(t, user1.Token, user2.Token)
}

// MockLoginController for testing context functionality
type MockLoginController struct {
	shouldReturnError bool
	noBrowserCalled   bool
	lastNoBrowserFlag bool
}

func (m *MockLoginController) AuthenticateToOktetoCluster(ctx context.Context, oktetoURL, token string) (*types.User, error) {
	return m.AuthenticateToOktetoClusterWithOptions(ctx, oktetoURL, token, false)
}

func (m *MockLoginController) AuthenticateToOktetoClusterWithOptions(ctx context.Context, oktetoURL, token string, noBrowser bool) (*types.User, error) {
	m.noBrowserCalled = true
	m.lastNoBrowserFlag = noBrowser
	
	if m.shouldReturnError {
		return nil, assert.AnError
	}
	
	if token != "" {
		return &types.User{Token: token}, nil
	}
	
	return &types.User{Token: "mock-token"}, nil
}

func TestMockLoginController(t *testing.T) {
	mock := &MockLoginController{}
	ctx := context.Background()

	// Test with token
	user, err := mock.AuthenticateToOktetoClusterWithOptions(ctx, "https://test.okteto.com", "test-token", true)
	assert.NoError(t, err)
	assert.Equal(t, "test-token", user.Token)
	assert.True(t, mock.noBrowserCalled)
	assert.True(t, mock.lastNoBrowserFlag)

	// Reset mock
	mock.noBrowserCalled = false
	mock.lastNoBrowserFlag = false

	// Test without token
	user, err = mock.AuthenticateToOktetoClusterWithOptions(ctx, "https://test.okteto.com", "", false)
	assert.NoError(t, err)
	assert.Equal(t, "mock-token", user.Token)
	assert.True(t, mock.noBrowserCalled)
	assert.False(t, mock.lastNoBrowserFlag)
}