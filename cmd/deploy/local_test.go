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

package deploy

import (
	"context"
	"net"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestDeployNotRemovingEnvFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	_, err := fs.Create(".env")
	require.NoError(t, err)
	opts := &Options{
		Manifest: &model.Manifest{
			Deploy: &model.DeployInfo{},
		},
	}
	localDeployer := localDeployer{
		ConfigMapHandler: &fakeCmapHandler{},
		Fs:               fs,
	}
	err = localDeployer.runDeploySection(context.Background(), opts)
	assert.NoError(t, err)
	_, err = fs.Stat(".env")
	require.NoError(t, err)

}

type fakeK8sProvider struct {
	err error
}

func (f *fakeK8sProvider) Provide(_ *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return nil, nil, nil
}

func Test_newLocalDeployer(t *testing.T) {
	dnsError := &net.DNSError{
		IsNotFound: true,
	}
	tests := []struct {
		name            string
		opts            *Options
		fakePortGetter  func(string) (int, error)
		fakeCmapHandler configMapHandler
		fakeK8sProvider *fakeK8sProvider
		fakeKubeConfig  *fakeKubeConfig
		expectedErr     error
		isUserErr       bool
	}{
		{
			name: "error providing k8s client when no opts.Name",
			opts: &Options{},
			fakeK8sProvider: &fakeK8sProvider{
				err: assert.AnError,
			},
			expectedErr: assert.AnError,
		},
		{
			name: "error getting new proxy: port not found",
			opts: &Options{
				Name: "test",
			},
			fakePortGetter: func(string) (int, error) {
				return 0, dnsError
			},
			expectedErr: dnsError,
			isUserErr:   true,
		},
		{
			name: "error getting new proxy: any error",
			opts: &Options{
				Name: "test",
			},
			fakePortGetter: func(string) (int, error) {
				return 0, assert.AnError
			},
			expectedErr: assert.AnError,
		},
		{
			name: "return localDeployer",
			opts: &Options{
				Name: "test",
			},
			fakeCmapHandler: &fakeCmapHandler{},
			fakeK8sProvider: &fakeK8sProvider{},
			fakePortGetter: func(string) (int, error) {
				return 123456, nil
			},
			fakeKubeConfig: &fakeKubeConfig{
				config: &rest.Config{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			got, err := newLocalDeployer(ctx, tt.opts, tt.fakeCmapHandler, tt.fakeK8sProvider, tt.fakeKubeConfig, tt.fakePortGetter)

			if tt.expectedErr == nil {
				require.NotNil(t, got)
			}
			require.ErrorIs(t, err, tt.expectedErr)

			_, ok := err.(oktetoErrors.UserError)
			require.True(t, (tt.isUserErr == ok))
		})
	}
}
