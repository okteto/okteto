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

package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestValidateEndpointMode(t *testing.T) {
	tests := []struct {
		name          string
		endpointMode  string
		expectedError bool
	}{
		{
			name:          "valid vip mode",
			endpointMode:  "vip",
			expectedError: false,
		},
		{
			name:          "valid dnsrr mode",
			endpointMode:  "dnsrr",
			expectedError: false,
		},
		{
			name:          "invalid mode",
			endpointMode:  "invalid",
			expectedError: true,
		},
		{
			name:          "empty mode",
			endpointMode:  "",
			expectedError: true,
		},
		{
			name:          "case sensitive - VIP should fail",
			endpointMode:  "VIP",
			expectedError: true,
		},
		{
			name:          "case sensitive - DNSRR should fail",
			endpointMode:  "DNSRR",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEndpointMode(tt.endpointMode)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), fmt.Sprintf("invalid value '%s'", tt.endpointMode))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStackUnmarshalEndpointModeVIP(t *testing.T) {
	stackYAML := `
services:
  frontend:
    image: bitnam/nginx
    ports:
      - "8080:8080"
    deploy:
      endpoint_mode: vip
`
	var stack Stack
	stack.IsCompose = true
	err := yaml.UnmarshalStrict([]byte(stackYAML), &stack)
	require.NoError(t, err)

	assert.Contains(t, stack.Services, "frontend")
	assert.Equal(t, "vip", stack.Services["frontend"].EndpointMode)
}

func TestStackUnmarshalEndpointModeDNSRR(t *testing.T) {
	stackYAML := `
services:
  frontend:
    image: bitnam/nginx
    ports:
      - "8080:8080"
    deploy:
      endpoint_mode: dnsrr
`
	var stack Stack
	stack.IsCompose = true
	err := yaml.UnmarshalStrict([]byte(stackYAML), &stack)
	require.NoError(t, err)

	assert.Contains(t, stack.Services, "frontend")
	assert.Equal(t, "dnsrr", stack.Services["frontend"].EndpointMode)
}

func TestStackUnmarshalEndpointModeDefault(t *testing.T) {
	stackYAML := `
services:
  frontend:
    image: bitnam/nginx
    ports:
      - "8080:8080"
`
	var stack Stack
	stack.IsCompose = true
	err := yaml.UnmarshalStrict([]byte(stackYAML), &stack)
	require.NoError(t, err)

	assert.Contains(t, stack.Services, "frontend")
	// Should be empty when not specified (no modifications to manifest)
	assert.Equal(t, "", stack.Services["frontend"].EndpointMode)
}

func TestStackUnmarshalEndpointModeInvalid(t *testing.T) {
	stackYAML := `
services:
  frontend:
    image: bitnam/nginx
    ports:
      - "8080:8080"
    deploy:
      endpoint_mode: invalid
`
	var stack Stack
	stack.IsCompose = true
	err := yaml.UnmarshalStrict([]byte(stackYAML), &stack)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "services[frontend].deploy.endpoint_mode")
	assert.Contains(t, err.Error(), "invalid value 'invalid'")
}

func TestStackUnmarshalEndpointModeEmptyDeploy(t *testing.T) {
	stackYAML := `
services:
  frontend:
    image: bitnam/nginx
    ports:
      - "8080:8080"
    deploy: {}
`
	var stack Stack
	stack.IsCompose = true
	err := yaml.UnmarshalStrict([]byte(stackYAML), &stack)
	require.NoError(t, err)

	assert.Contains(t, stack.Services, "frontend")
	// Should be empty when deploy section exists but endpoint_mode is not specified
	assert.Equal(t, "", stack.Services["frontend"].EndpointMode)
}

func TestStackUnmarshalWithOtherDeployFields(t *testing.T) {
	stackYAML := `
services:
  frontend:
    image: bitnam/nginx
    ports:
      - "8080:8080"
    deploy:
      replicas: 3
      endpoint_mode: dnsrr
      restart_policy:
        condition: on-failure
        max_attempts: 3
`
	var stack Stack
	stack.IsCompose = true
	err := yaml.UnmarshalStrict([]byte(stackYAML), &stack)
	require.NoError(t, err)

	assert.Contains(t, stack.Services, "frontend")
	assert.Equal(t, "dnsrr", stack.Services["frontend"].EndpointMode)
	assert.Equal(t, int32(3), stack.Services["frontend"].Replicas)
	assert.Equal(t, int32(3), stack.Services["frontend"].BackOffLimit)
}