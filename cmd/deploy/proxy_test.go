package deploy

import (
	"encoding/json"
	"net"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
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

type fakePortGetter struct {
	port int
	err  error
}

func (pg fakePortGetter) Get(_ string) (int, error) {
	if pg.err != nil {
		return 0, pg.err
	}
	return pg.port, nil
}

func Test_NewProxy(t *testing.T) {
	notFoundErr := &net.DNSError{
		IsNotFound: true,
	}

	tests := []struct {
		name           string
		portGetter     fakePortGetter
		fakeKubeconfig *fakeKubeConfig
		expectedProxy  *Proxy
		expectedErr    error
	}{
		{
			name: "err getting port, DNS not found error",
			portGetter: fakePortGetter{
				err: notFoundErr,
			},
			expectedErr: oktetoErrors.UserError{
				E:    notFoundErr,
				Hint: "Review your /etc/hosts configuration, make sure there is an entry for localhost",
			},
		},
		{
			name: "err getting port, any error",
			portGetter: fakePortGetter{
				err: assert.AnError,
			},
			expectedErr: assert.AnError,
		},
		{
			name: "err reading kubeconfig",
			portGetter: fakePortGetter{
				port: 12345,
			},
			fakeKubeconfig: &fakeKubeConfig{
				errRead: assert.AnError,
			},
			expectedErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProxy(tt.fakeKubeconfig, tt.portGetter.Get)
			require.Equal(t, tt.expectedProxy, got)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
