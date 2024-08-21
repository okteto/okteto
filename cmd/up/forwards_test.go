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
	"github.com/okteto/okteto/pkg/vars"
	"github.com/stretchr/testify/assert"
)

type varManagerLogger struct{}

func (varManagerLogger) Yellow(_ string, _ ...interface{}) {}
func (varManagerLogger) AddMaskedWord(_ string)            {}

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
		upContext   *upContext
		name        string
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
		clientProvider         okteto.K8sClientProvider
		expected               error
		name                   string
		OktetoExecuteSSHEnvVar string
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
			varManager := vars.NewVarsManager(&varManagerLogger{})
			varManager.AddLocalVar(model.OktetoExecuteSSHEnvVar, tc.OktetoExecuteSSHEnvVar)
			vars.GlobalVarManager = varManager

			okteto.CurrentStore = &okteto.ContextStore{
				CurrentContext: "test",
				Contexts: map[string]*okteto.Context{
					"test": {},
				},
			}

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
				varManager:        varManager,
			}

			err := up.forwards(context.Background())
			assert.Equal(t, tc.expected, err)
		})
	}
}

func TestSSHForwarss(t *testing.T) {
	tt := []struct {
		clientProvider okteto.K8sClientProvider
		expected       error
		name           string
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
