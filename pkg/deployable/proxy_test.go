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

package deployable

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

var (
	ph = &proxyHandler{}
)

type fakeKubeConfig struct {
	config      *rest.Config
	errOnModify error
	errRead     error
}

func (f *fakeKubeConfig) Read() (*rest.Config, error) {
	if f.errRead != nil {
		return nil, f.errRead
	}
	return f.config, nil
}

func (fc *fakeKubeConfig) Modify(_ int, _, _ string) error {
	return fc.errOnModify
}

func Test_NewProxy(t *testing.T) {
	dnsErr := &net.DNSError{
		IsNotFound: true,
	}

	tests := []struct {
		expectedErr    error
		portGetter     func(string) (int, error)
		fakeKubeconfig *fakeKubeConfig
		expectedProxy  *Proxy
		name           string
	}{
		{
			name:        "err getting port, DNS not found error",
			portGetter:  func(string) (int, error) { return 0, dnsErr },
			expectedErr: dnsErr,
		},
		{
			name:        "err getting port, any error",
			portGetter:  func(string) (int, error) { return 0, assert.AnError },
			expectedErr: assert.AnError,
		},
		{
			name:       "err reading kubeconfig",
			portGetter: func(string) (int, error) { return 0, nil },
			fakeKubeconfig: &fakeKubeConfig{
				errRead: assert.AnError,
			},
			expectedErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProxy(tt.fakeKubeconfig, tt.portGetter)
			require.Equal(t, tt.expectedProxy, got)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
