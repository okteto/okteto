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
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	ph = &proxyHandler{}
)

func Test_TranslateInvalidResourceBody(t *testing.T) {
	t.Parallel()
	var tests = []struct {
		name string
		body []byte
	}{
		{
			name: "null body json.RawMessage",
			body: []byte(``),
		},
		{
			name: "correct body json.RawMessage",
			body: []byte(`{"kind":"Secret","apiVersion":"v1","metadata":{"name":"sh.helm.release.v1.movies.v6"},"type":"helm.sh/release.v1"}`),
		},
		{
			name: "incorrect body typemeta",
			body: []byte(`{"kind": {"key": "value"},"apiVersion":"v1","metadata":{"name":"sh.helm.release.v1.movies.v6"},"type":"helm.sh/release.v1"}`),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ph.translateBody(tt.body)
			assert.NoError(t, err)
		})
	}
}

func Test_TranslateInvalidResourceSpec(t *testing.T) {
	invalidResourceSpec := map[string]json.RawMessage{
		"spec": []byte(`{"selector": "invalid value"}`),
	}
	assert.NoError(t, ph.translateDeploymentSpec(invalidResourceSpec))
	assert.NoError(t, ph.translateStatefulSetSpec(invalidResourceSpec))
	assert.NoError(t, ph.translateReplicationControllerSpec(invalidResourceSpec))
	assert.NoError(t, ph.translateReplicaSetSpec(invalidResourceSpec))
	assert.NoError(t, ph.translateDaemonSetSpec(invalidResourceSpec))

	assert.NoError(t, ph.translateJobSpec(map[string]json.RawMessage{
		"spec": []byte(`{"parallelism": "invalid value"}`),
	}))

	assert.NoError(t, ph.translateCronJobSpec(map[string]json.RawMessage{
		"spec": []byte(`{"schedule": 1}`),
	}))
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
