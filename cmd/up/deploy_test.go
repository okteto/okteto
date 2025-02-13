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

package up

import (
	"context"
	"strconv"
	"testing"

	"github.com/okteto/okteto/internal/test"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
)

func TestNewDevEnvDeployer(t *testing.T) {
	tests := []struct {
		name               string
		opts               *Options
		autoDeploy         bool
		expectedMustDeploy bool
		manifestName       string
		namespace          string
	}{
		{
			name:               "Deploy flag true, autoDeploy false",
			opts:               &Options{Deploy: true},
			autoDeploy:         false,
			expectedMustDeploy: true,
			manifestName:       "app1",
			namespace:          "ns1",
		},
		{
			name:               "Deploy flag false, autoDeploy false",
			opts:               &Options{Deploy: false},
			autoDeploy:         false,
			expectedMustDeploy: false,
			manifestName:       "app2",
			namespace:          "ns2",
		},
		{
			name:               "Deploy flag false, autoDeploy true",
			opts:               &Options{Deploy: false},
			autoDeploy:         true,
			expectedMustDeploy: true,
			manifestName:       "app3",
			namespace:          "ns3",
		},
		{
			name:               "Deploy flag true, autoDeploy true",
			opts:               &Options{Deploy: true},
			autoDeploy:         true,
			expectedMustDeploy: true,
			manifestName:       "app4",
			namespace:          "ns4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(OktetoAutoDeployEnvVar, strconv.FormatBool(tt.autoDeploy))

			up := &upContext{
				Manifest: &model.Manifest{
					Name: tt.manifestName,
				},
				K8sClientProvider: test.NewFakeK8sProvider(),
			}

			okCtx := &okteto.Context{
				Namespace: tt.namespace,
			}
			ioCtrl := io.NewIOController()
			k8sLogger := io.NewK8sLogger()

			deployer := NewDevEnvDeployerManager(up, tt.opts, okCtx, ioCtrl, k8sLogger)

			assert.Equal(t, tt.expectedMustDeploy, deployer.mustDeploy, "mustDeploy should be set correctly")
			assert.Equal(t, tt.manifestName, deployer.devenvName, "devenvName should come from up.Manifest.Name")
			assert.Equal(t, tt.namespace, deployer.ns, "namespace should be set from okCtx.Namespace")
			assert.NotNil(t, deployer.ioCtrl, "ioCtrl should not be nil")
			assert.NotNil(t, deployer.k8sClientProvider, "k8sClientProvider should not be nil")
			assert.NotNil(t, deployer.deployStrategy, "deployStrategy should be initialized")
		})
	}
}

type fakeDeployStrategy struct {
	deployed bool
	err      error
}

// Deploy implements the devEnvEnvDeployStrategy interface
func (f *fakeDeployStrategy) Deploy(context.Context) error {
	f.deployed = true
	return f.err
}

func TestDeployIfNeeded(t *testing.T) {
	type input struct {
		isOkteto             bool
		mustDeploy           bool
		isDeployedApp        bool
		k8sClientProviderErr error
		deployErr            error
	}
	type expected struct {
		isDeploymentExpected bool
		expectedErr          error
	}
	tests := []struct {
		name     string
		input    input
		expected expected
	}{
		{
			name:     "not okteto context",
			input:    input{isOkteto: false},
			expected: expected{isDeploymentExpected: false, expectedErr: nil},
		},
		{
			name:     "error creating k8s client",
			input:    input{isOkteto: true, k8sClientProviderErr: assert.AnError},
			expected: expected{isDeploymentExpected: false, expectedErr: assert.AnError},
		},
		{
			name:     "must deploy: true // isDeployedApp: false",
			input:    input{isOkteto: true, k8sClientProviderErr: nil, mustDeploy: true},
			expected: expected{isDeploymentExpected: true, expectedErr: nil},
		},
		{
			name:     "must deploy: true // isDeployedApp: true",
			input:    input{isOkteto: true, k8sClientProviderErr: nil, mustDeploy: true, isDeployedApp: true},
			expected: expected{isDeploymentExpected: true, expectedErr: nil},
		},
		{
			name:     "must deploy: false // isDeployedApp: true",
			input:    input{isOkteto: true, k8sClientProviderErr: nil, mustDeploy: false, isDeployedApp: true},
			expected: expected{isDeploymentExpected: false, expectedErr: nil},
		},
		{
			name:     "must deploy: false // isDeployedApp: false",
			input:    input{isOkteto: true, k8sClientProviderErr: nil, mustDeploy: false},
			expected: expected{isDeploymentExpected: true, expectedErr: nil},
		},
		{
			name:     "deploy but error deploying",
			input:    input{isOkteto: true, mustDeploy: false, deployErr: assert.AnError},
			expected: expected{isDeploymentExpected: true, expectedErr: assert.AnError},
		},
		{
			name:     "must deploy: false // isDeployedApp: false",
			input:    input{isOkteto: true, mustDeploy: true, deployErr: oktetoErrors.ErrManifestFoundButNoDeployAndDependenciesCommands},
			expected: expected{isDeploymentExpected: true, expectedErr: nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployer := &devEnvDeployerManager{
				okCtx: &okteto.Context{
					IsOkteto: tt.input.isOkteto,
				},
				k8sClientProvider: &test.FakeK8sProvider{
					ErrProvide: tt.input.k8sClientProviderErr,
				},
				mustDeploy: tt.input.mustDeploy,
				isDevEnvDeployed: func(context.Context, string, string, kubernetes.Interface) bool {
					return tt.input.isDeployedApp
				},
				deployStrategy: &fakeDeployStrategy{
					err: tt.input.deployErr,
				},
				ioCtrl: io.NewIOController(),
			}
			err := deployer.DeployIfNeeded(context.Background())
			assert.ErrorIs(t, tt.expected.expectedErr, err)
			assert.Equal(t, tt.expected.isDeploymentExpected, deployer.deployStrategy.(*fakeDeployStrategy).deployed)
		})
	}

}
