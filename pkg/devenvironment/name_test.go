package devenvironment

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestInferName(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name             string
		getRepositoryURL func(string) (string, error)
		devEnvs          []runtime.Object
		ns               string
		manifestPath     string
		cwd              string
		expectedName     string
	}{
		{
			name: "without-repository-url",
			getRepositoryURL: func(s string) (string, error) {
				return "", assert.AnError
			},
			devEnvs:      []runtime.Object{},
			ns:           "test",
			manifestPath: "my-manifest/okteto.yml",
			cwd:          "/tmp/my-dev-env",
			expectedName: "my-dev-env",
		},
		{
			name: "without-dev-envs",
			getRepositoryURL: func(s string) (string, error) {
				return "https://github.com/test-user/my-dev-env-repository.git", nil
			},
			devEnvs:      []runtime.Object{},
			ns:           "test",
			manifestPath: "my-manifest/okteto.yml",
			cwd:          "/tmp/my-dev-env",
			expectedName: "my-dev-env-repository",
		},
		{
			name: "without-matching-criteria-dev-envs",
			getRepositoryURL: func(s string) (string, error) {
				return "https://github.com/test-user/my-dev-env-repository.git", nil
			},
			devEnvs:      getDevEnvironmentConfigMaps(),
			ns:           "test",
			manifestPath: "my-manifest/okteto.yml",
			cwd:          "/tmp/my-dev-env",
			expectedName: "my-dev-env-repository",
		},
		{
			name: "with-matching-criteria-for-one-dev-envs",
			getRepositoryURL: func(s string) (string, error) {
				return "https://github.com/test/single-repo.git", nil
			},
			devEnvs:      getDevEnvironmentConfigMaps(),
			ns:           "test",
			manifestPath: "my-manifest/okteto.yml",
			cwd:          "/tmp/my-dev-env",
			expectedName: "single dev name",
		},
		{
			name: "with-matching-criteria-for-multiple-dev-envs",
			getRepositoryURL: func(s string) (string, error) {
				return "https://github.com/test/multiple-repo.git", nil
			},
			devEnvs:      getDevEnvironmentConfigMaps(),
			ns:           "test",
			manifestPath: "my-manifest-multiple/okteto.yml",
			cwd:          "/tmp/my-dev-env",
			expectedName: "multiple dev name 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewSimpleClientset(tt.devEnvs...)
			inferer := NameInferer{
				k8s:              c,
				getRepositoryURL: tt.getRepositoryURL,
			}

			result := inferer.InferName(ctx, tt.cwd, tt.ns, tt.manifestPath)
			require.Equal(t, tt.expectedName, result)
		})
	}
}

func getDevEnvironmentConfigMaps() []runtime.Object {
	return []runtime.Object{
		&apiv1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "single-dev-name",
				Namespace: "test",
				Labels: map[string]string{
					model.GitDeployLabel: "true",
				},
			},
			Data: map[string]string{
				"name":       "single dev name",
				"repository": "https://github.com/test/single-repo.git",
				"filename":   "my-manifest/okteto.yml",
			},
		},
		&apiv1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "multiple-dev-name-1",
				Namespace: "test",
				Labels: map[string]string{
					model.GitDeployLabel: "true",
				},
			},
			Data: map[string]string{
				"name":       "multiple dev name 1",
				"repository": "https://github.com/test/multiple-repo.git",
				"filename":   "my-manifest-multiple/okteto.yml",
			},
		},
		&apiv1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "multiple-dev-name-2",
				Namespace: "test",
				Labels: map[string]string{
					model.GitDeployLabel: "true",
				},
			},
			Data: map[string]string{
				"name":       "multiple dev name 2",
				"repository": "https://github.com/test/multiple-repo.git",
				"filename":   "my-manifest-multiple/okteto.yml",
			},
		},
		&apiv1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "multiple-dev-name-3",
				Namespace: "test",
				Labels: map[string]string{
					model.GitDeployLabel: "true",
				},
			},
			Data: map[string]string{
				"name":       "multiple dev name 3",
				"repository": "https://github.com/test/multiple-repo.git",
				"filename":   "my-manifest-multiple/okteto.yml",
			},
		},
	}
}
