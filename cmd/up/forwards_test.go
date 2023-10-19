package up

import (
	"context"
	"fmt"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/stretchr/testify/assert"
)

func TestGlobalForwarderStartsWhenRequired(t *testing.T) {
	t.Parallel()
	var tests = []struct {
		name             string
		globalFwdSection []forward.GlobalForward
		expectedAnswer   bool
	}{
		{
			name: "is needed global forwarding",
			globalFwdSection: []forward.GlobalForward{
				{
					Local:  8080,
					Remote: 8080,
				},
			},
			expectedAnswer: true,
		},
		{
			name:             "not needed global forwarding",
			globalFwdSection: []forward.GlobalForward{},
			expectedAnswer:   false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			answer := isNeededGlobalForwarder(tt.globalFwdSection)
			assert.Equal(t, answer, tt.expectedAnswer)
		})
	}
}

func TestGlobalForwarderAddsProperlyPortsToForward(t *testing.T) {
	f := ssh.NewForwardManager(context.Background(), ":8080", "0.0.0.0", "0.0.0.0", nil, "test")

	var tests = []struct {
		name        string
		upContext   *upContext
		expectedErr bool
	}{
		{
			name: "add one global forwarder",
			upContext: &upContext{
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{
						{
							Local:  8080,
							Remote: 8080,
						},
					},
				},
				Forwarder: f,
			},
			expectedErr: false,
		},
		{
			name: "add two global forwarder",
			upContext: &upContext{
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{
						{
							Local:       8081,
							ServiceName: "api",
							Remote:      8080,
						},
						{
							Local:  8082,
							Remote: 8080,
						},
					},
				},
				Forwarder: f,
			},
			expectedErr: false,
		},
		{
			name: "add none global forwarder",
			upContext: &upContext{
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{},
				},
				Forwarder: f,
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := addGlobalForwards(tt.upContext)
			if !tt.expectedErr {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
			}
		})
	}
}

func TestForwards(t *testing.T) {
	tt := []struct {
		name                   string
		OktetoExecuteSSHEnvVar string
		clientProvider         okteto.K8sClientProvider
		expected               error
	}{
		{
			name:                   "fakeClientProvider error",
			OktetoExecuteSSHEnvVar: "false",
			clientProvider: &test.FakeK8sProvider{
				ErrProvide: assert.AnError,
			},
			expected: assert.AnError,
		},
		{
			name:                   "fakeClientProvider error",
			OktetoExecuteSSHEnvVar: "false",
			clientProvider:         test.NewFakeK8sProvider(),
			expected:               fmt.Errorf("port %d is listed multiple times, please check your configuration", 8080),
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			up := &upContext{
				Dev: &model.Dev{
					Forward: []forward.Forward{
						{
							Local:  8080,
							Remote: 8080,
						},
						{
							Local:  8080,
							Remote: 8080,
						},
					},
				},
				K8sClientProvider: tc.clientProvider,
			}
			t.Setenv(model.OktetoExecuteSSHEnvVar, tc.OktetoExecuteSSHEnvVar)
			err := up.forwards(context.Background())
			assert.Equal(t, tc.expected, err)
		})
	}
}

func TestSSHForwarss(t *testing.T) {
	tt := []struct {
		name           string
		clientProvider okteto.K8sClientProvider
		expected       error
	}{
		{
			name: "fakeClientProvider error",
			clientProvider: &test.FakeK8sProvider{
				ErrProvide: assert.AnError,
			},
			expected: assert.AnError,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			up := &upContext{
				Dev: &model.Dev{
					Forward: []forward.Forward{
						{
							Local:  8080,
							Remote: 8080,
						},
						{
							Local:  8080,
							Remote: 8080,
						},
					},
				},
				K8sClientProvider: tc.clientProvider,
			}
			err := up.sshForwards(context.Background())
			assert.ErrorIs(t, tc.expected, err)
		})
	}
}
