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

package build

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/moby/buildkit/session/auth"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_isErrCredentialsHelperNotAccessible(t *testing.T) {
	tests := []struct {
		err      error
		name     string
		expected bool
	}{
		{
			name:     "credential not accessible error ",
			err:      errors.New("error getting credentials: something resolves to executable in current directory (whatever)"),
			expected: true,
		},
		{
			name:     "credential not accessible error",
			err:      errors.New("error getting credentials: foo executable file not found in $PATH (bar)"),
			expected: true,
		},
		{
			name:     "not a credential not accessible error",
			err:      errors.New("error getting credentials: other error message"),
			expected: false,
		},
		{
			name:     "not a credential not accessible error",
			err:      errors.New("error: resolves to executable in current directory"),
			expected: false,
		},
		{
			name:     "not a credential not accessible error",
			err:      errors.New("a totally different error message"),
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, isErrCredentialsHelperNotAccessible(tt.err), tt.expected)
		})
	}
}

func Test_GetAuthConfig_OmisionIfNeeded(t *testing.T) {
	config := &configfile.ConfigFile{
		AuthConfigs: map[string]types.AuthConfig{
			"https://index.docker.io/v1/": {},
		},
		CredentialsStore: "okteto-fake", // resolves to binary named docker-credential-okteto-fake, which shouldn't be present at $PATH
	}
	_, err := config.GetAuthConfig("https://index.docker.io/v1/")
	require.Error(t, err)
	t.Logf("error is: %q", err)
	require.True(t, isErrCredentialsHelperNotAccessible(err))
}

type fakeContext struct{}

func (fakeContext) isOktetoContext() bool                 { return true }
func (fakeContext) getOktetoClientCfg() *okteto.ClientCfg { return nil }
func (fakeContext) getExternalRegistryCreds(string, bool, *okteto.Client) (string, string, error) {
	return "", "", nil
}

func TestGetOktetoCredentials(t *testing.T) {
	tt := []struct {
		err      error
		expected *auth.CredentialsResponse
		name     string
		user     string
		pass     string
	}{
		{
			name: "okteto credentials",
			user: "test",
			pass: "test",
			err:  nil,
			expected: &auth.CredentialsResponse{
				Username: "test",
				Secret:   "test",
			},
		},
		{
			name:     "error getting okteto credentials",
			err:      assert.AnError,
			expected: &auth.CredentialsResponse{},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ap := &authProvider{
				externalAuth: func(string, bool, *okteto.Client) (string, string, error) {
					return tc.user, tc.pass, tc.err
				},
				authContext: &fakeContext{},
			}
			creds := ap.getOktetoCredentials("", nil)
			require.Equal(t, tc.expected, creds)
		})
	}
}

func TestCredentials(t *testing.T) {
	oktetoRegistry = "okteto.registry.com"

	tt := []struct {
		credRequest       *auth.CredentialsRequest
		expected          *auth.CredentialsResponse
		name              string
		envVarValue       string
		envVarSet         bool
		localCredentials  bool
		oktetoCredentials bool
	}{
		{
			name: "okteto registry",
			credRequest: &auth.CredentialsRequest{
				Host: "okteto.registry.com",
			},
			expected: &auth.CredentialsResponse{
				Username: "okteto-registry",
				Secret:   "okteto-registry",
			},
		},

		{
			name: "local env var: true / local credentials: true / okteto credentials: false",
			credRequest: &auth.CredentialsRequest{
				Host: "registry-1.docker.io",
			},
			envVarSet:         true,
			envVarValue:       "true",
			localCredentials:  true,
			oktetoCredentials: false,
			expected: &auth.CredentialsResponse{
				Username: "local",
				Secret:   "local",
			},
		},
		{
			name: "local env var: true / local credentials: false / okteto credentials: true",
			credRequest: &auth.CredentialsRequest{
				Host: "registry-1.docker.io",
			},
			envVarSet:         true,
			envVarValue:       "true",
			localCredentials:  false,
			oktetoCredentials: true,
			expected: &auth.CredentialsResponse{
				Username: "okteto",
				Secret:   "okteto",
			},
		},
		{
			name: "local env var: true / local credentials: true / okteto credentials: true",
			credRequest: &auth.CredentialsRequest{
				Host: "registry-1.docker.io",
			},
			envVarSet:         true,
			envVarValue:       "true",
			localCredentials:  true,
			oktetoCredentials: true,
			expected: &auth.CredentialsResponse{
				Username: "local",
				Secret:   "local",
			},
		},
		{
			name: "local env var: true / local credentials: false / okteto credentials: false",
			credRequest: &auth.CredentialsRequest{
				Host: "registry-1.docker.io",
			},
			envVarSet:         true,
			envVarValue:       "true",
			localCredentials:  false,
			oktetoCredentials: false,
			expected: &auth.CredentialsResponse{
				Username: "",
				Secret:   "",
			},
		},

		{
			name: "local env var: false / local credentials: true / okteto credentials: false",
			credRequest: &auth.CredentialsRequest{
				Host: "registry-1.docker.io",
			},
			envVarSet:         true,
			envVarValue:       "false",
			localCredentials:  true,
			oktetoCredentials: false,
			expected: &auth.CredentialsResponse{
				Username: "",
				Secret:   "",
			},
		},
		{
			name: "local env var: false / local credentials: false / okteto credentials: true",
			credRequest: &auth.CredentialsRequest{
				Host: "registry-1.docker.io",
			},
			envVarSet:         true,
			envVarValue:       "false",
			localCredentials:  false,
			oktetoCredentials: true,
			expected: &auth.CredentialsResponse{
				Username: "okteto",
				Secret:   "okteto",
			},
		},
		{
			name: "local env var: false / local credentials: true / okteto credentials: true",
			credRequest: &auth.CredentialsRequest{
				Host: "registry-1.docker.io",
			},
			envVarSet:         true,
			envVarValue:       "false",
			localCredentials:  true,
			oktetoCredentials: true,
			expected: &auth.CredentialsResponse{
				Username: "okteto",
				Secret:   "okteto",
			},
		},
		{
			name: "local env var: false / local credentials: false / okteto credentials: false",
			credRequest: &auth.CredentialsRequest{
				Host: "registry-1.docker.io",
			},
			envVarSet:         true,
			envVarValue:       "false",
			localCredentials:  false,
			oktetoCredentials: false,
			expected: &auth.CredentialsResponse{
				Username: "",
				Secret:   "",
			},
		},

		{
			name: "local env var: unset / local credentials: true / okteto credentials: false",
			credRequest: &auth.CredentialsRequest{
				Host: "registry-1.docker.io",
			},
			envVarSet:         false,
			localCredentials:  true,
			oktetoCredentials: false,
			expected: &auth.CredentialsResponse{
				Username: "local",
				Secret:   "local",
			},
		},
		{
			name: "local env var: unset / local credentials: false / okteto credentials: true",
			credRequest: &auth.CredentialsRequest{
				Host: "registry-1.docker.io",
			},
			envVarSet:         false,
			localCredentials:  false,
			oktetoCredentials: true,
			expected: &auth.CredentialsResponse{
				Username: "okteto",
				Secret:   "okteto",
			},
		},
		{
			name: "local env var: unset / local credentials: true / okteto credentials: true",
			credRequest: &auth.CredentialsRequest{
				Host: "registry-1.docker.io",
			},
			envVarSet:         false,
			localCredentials:  true,
			oktetoCredentials: true,
			expected: &auth.CredentialsResponse{
				Username: "local",
				Secret:   "local",
			},
		},
		{
			name: "local env var: unset / local credentials: false / okteto credentials: false",
			credRequest: &auth.CredentialsRequest{
				Host: "registry-1.docker.io",
			},
			envVarSet:         false,
			localCredentials:  false,
			oktetoCredentials: false,
			expected: &auth.CredentialsResponse{
				Username: "",
				Secret:   "",
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ap := &authProvider{
				config: &configfile.ConfigFile{
					AuthConfigs: map[string]types.AuthConfig{
						"okteto.registry.com": {
							Username: "okteto-registry",
							Password: "okteto-registry",
						},
					},
				},
				externalAuth: func(string, bool, *okteto.Client) (string, string, error) {
					return "", "", nil
				},
				newOktetoClient: func(cfg *okteto.ClientCfg, opts ...okteto.Option) (*okteto.Client, error) {
					return &okteto.Client{}, nil
				},
				authContext: &fakeContext{},
			}
			if tc.envVarSet {
				t.Setenv(oktetoLocalRegistryStoreEnabledEnvVarKey, tc.envVarValue)
			}

			if tc.localCredentials {
				ap.config.AuthConfigs["https://index.docker.io/v1/"] = types.AuthConfig{
					Username: "local",
					Password: "local",
				}
			}

			if tc.oktetoCredentials {
				ap.externalAuth = func(string, bool, *okteto.Client) (string, string, error) {
					return "okteto", "okteto", nil
				}
			}

			creds, err := ap.Credentials(context.Background(), tc.credRequest)
			require.NoError(t, err)
			require.Equal(t, tc.expected, creds)
		})
	}
}
