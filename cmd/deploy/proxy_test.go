package deploy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	ph = &proxyHandler{}
)

func Test_TranslateInvalidResourceBody(t *testing.T) {
	var tests = []struct {
		name       string
		body       []byte
		isCreation bool
	}{
		{
			name:       "null body json.RawMessage POST",
			body:       []byte(``),
			isCreation: true,
		},
		{
			name:       "correct body json.RawMessage POST",
			body:       []byte(`{"kind":"Secret","apiVersion":"v1","metadata":{"name":"sh.helm.release.v1.movies.v6"},"type":"helm.sh/release.v1"}`),
			isCreation: true,
		},
		{
			name:       "incorrect body typemeta POST",
			body:       []byte(`{"kind": {"key": "value"},"apiVersion":"v1","metadata":{"name":"sh.helm.release.v1.movies.v6"},"type":"helm.sh/release.v1"}`),
			isCreation: true,
		},
		{
			name:       "null body json.RawMessage PUT",
			body:       []byte(``),
			isCreation: false,
		},
		{
			name:       "correct body json.RawMessage PUT",
			body:       []byte(`{"kind":"Secret","apiVersion":"v1","metadata":{"name":"sh.helm.release.v1.movies.v6"},"type":"helm.sh/release.v1"}`),
			isCreation: false,
		},
		{
			name:       "incorrect body typemeta PUT",
			body:       []byte(`{"kind": {"key": "value"},"apiVersion":"v1","metadata":{"name":"sh.helm.release.v1.movies.v6"},"type":"helm.sh/release.v1"}`),
			isCreation: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ph.translateBody(tt.body, tt.isCreation)
			assert.NoError(t, err)
		})
	}
}

func Test_TranslateInvalidResourceSpec(t *testing.T) {
	invalidResourceSpec := map[string]json.RawMessage{
		"spec": []byte(`{"selector": "invalid value"}`),
	}
	assert.NoError(t, ph.translateDeploymentSpec(invalidResourceSpec))
	assert.NoError(t, ph.translateStatefulSetSpec(invalidResourceSpec, true))
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
