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
		name           string
		portGetter     func(string) (int, error)
		fakeKubeconfig *fakeKubeConfig
		expectedProxy  *Proxy
		expectedErr    error
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
