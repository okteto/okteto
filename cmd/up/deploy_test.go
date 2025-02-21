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
	"testing"
	"time"

	"github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
)

type fakeDeployer struct {
	deployed bool
	err      error
	tracked  bool
}

// Deploy implements the devEnvEnvDeployStrategy interface
func (f *fakeDeployer) Run(context.Context, *deploy.Options) error {
	f.deployed = true
	return f.err
}

func (f *fakeDeployer) TrackDeploy(*model.Manifest, bool, time.Time, error) {
	f.tracked = true
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
			fakeDeployer := &fakeDeployer{err: tt.input.deployErr}
			deployer := &devEnvDeployerManager{
				ioCtrl: io.NewIOController(),
				k8sClientProvider: &test.FakeK8sProvider{
					ErrProvide: tt.input.k8sClientProviderErr,
				},
				isDevEnvDeployed: func(ctx context.Context, name, namespace string, c kubernetes.Interface) bool {
					return tt.input.isDeployedApp
				},
				getDeployer: func(params deployParams) (deployer, error) {
					return fakeDeployer, nil
				},
			}
			deployParams := deployParams{
				deployFlag: tt.input.mustDeploy,
				okCtx:      &okteto.Context{IsOkteto: tt.input.isOkteto},
			}
			err := deployer.DeployIfNeeded(context.Background(), deployParams, &analytics.UpMetricsMetadata{})
			assert.ErrorIs(t, tt.expected.expectedErr, err)
			assert.Equal(t, tt.expected.isDeploymentExpected, fakeDeployer.deployed)
			assert.Equal(t, tt.expected.isDeploymentExpected, fakeDeployer.tracked)
		})
	}

}
