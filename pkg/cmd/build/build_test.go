package build

import (
	"path/filepath"
	"reflect"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
)

func Test_validateImage(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  error
	}{
		{
			name:  "okteto-dev-valid",
			image: "okteto.dev/image",
			want:  nil,
		},
		{
			name:  "okteto-dev-not-valid",
			image: "okteto.dev/image/hello",
			want:  oktetoErrors.UserError{},
		},
		{
			name:  "okteto-global-valid",
			image: "okteto.global/image",
			want:  nil,
		},
		{
			name:  "okteto-global-not-valid",
			image: "okteto.global/image/hello",
			want:  oktetoErrors.UserError{},
		},
		{
			name:  "not-okteto-image",
			image: "other/image/hello",
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateImage(tt.image); reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("build.validateImage = %v, want %v", reflect.TypeOf(got), reflect.TypeOf(tt.want))
			}
		})
	}
}

func Test_OptsFromManifest(t *testing.T) {
	tests := []struct {
		name           string
		serviceName    string
		buildInfo      *model.BuildInfo
		okGitCommitEnv string
		isOkteto       bool
		initialOpts    BuildOptions
		expected       BuildOptions
	}{
		{
			name:        "empty-values-is-okteto",
			serviceName: "service",
			buildInfo:   &model.BuildInfo{},
			isOkteto:    true,
			expected: BuildOptions{
				Tag: "okteto.dev/service:dev",
			},
		},
		{
			name:        "empty-values-is-not-okteto",
			serviceName: "service",
			buildInfo:   &model.BuildInfo{},
			isOkteto:    false,
			expected:    BuildOptions{},
		},
		{
			name:        "all-values-no-image",
			serviceName: "service",
			buildInfo: &model.BuildInfo{
				Context:    "service",
				Dockerfile: "CustomDockerfile",
				Target:     "build",
				CacheFrom:  []string{"cache-image"},
				Args: model.Environment{
					{
						Name:  "arg1",
						Value: "value1",
					},
				},
			},
			initialOpts: BuildOptions{
				OutputMode: "tty",
			},
			isOkteto: true,
			expected: BuildOptions{
				Tag:        "okteto.dev/service:dev",
				File:       filepath.Join("service", "CustomDockerfile"),
				Target:     "build",
				Path:       "service",
				CacheFrom:  []string{"cache-image"},
				BuildArgs:  []string{"arg1=value1"},
				OutputMode: "tty",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Namespace: "test",
						IsOkteto:  tt.isOkteto,
					},
				},
				CurrentContext: "test",
			}
			result := OptsFromManifest(tt.serviceName, tt.buildInfo, tt.initialOpts)
			assert.Equal(t, tt.expected, result)
		})
	}
}
