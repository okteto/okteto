package build

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
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
			buildInfo:   &model.BuildInfo{Name: "movies"},
			isOkteto:    true,
			expected: BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:okteto",
				BuildArgs:  []string{},
			},
		},
		{
			name:           "empty-values-is-okteto-local",
			serviceName:    "service",
			buildInfo:      &model.BuildInfo{Name: "movies"},
			okGitCommitEnv: "dev1235466",
			isOkteto:       true,
			expected: BuildOptions{
				Tag:        "okteto.dev/movies-service:okteto",
				OutputMode: oktetoLog.TTYFormat,
				BuildArgs:  []string{},
			},
		},
		{
			name:           "empty-values-is-okteto-pipeline",
			serviceName:    "service",
			buildInfo:      &model.BuildInfo{Name: "movies"},
			okGitCommitEnv: "1235466",
			isOkteto:       true,
			expected: BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:114921fe985b5f874c8d312b0a098959da6d119209c9d1e42a89c4309569692d",
				BuildArgs:  []string{},
			},
		},
		{
			name:        "empty-values-is-okteto-pipeline-withArgs",
			serviceName: "service",
			buildInfo: &model.BuildInfo{
				Name: "movies",
				Args: model.Environment{
					{
						Name:  "arg1",
						Value: "value1",
					},
				}},
			okGitCommitEnv: "1235466",
			isOkteto:       true,
			expected: BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:c0776074a88fa37835b1dfa67365b6a6b08b11c4cf49a9d42524ea9797959e58",
				BuildArgs:  []string{"arg1=value1"},
			},
		},
		{
			name:        "empty-values-is-not-okteto",
			serviceName: "service",
			buildInfo:   &model.BuildInfo{Name: "movies"},
			isOkteto:    false,
			expected: BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				BuildArgs:  []string{},
			},
		},
		{
			name:        "all-values-no-image",
			serviceName: "service",
			buildInfo: &model.BuildInfo{
				Name:       "movies",
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
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:okteto",
				File:       filepath.Join("service", "CustomDockerfile"),
				Target:     "build",
				Path:       "service",
				CacheFrom:  []string{"cache-image"},
				BuildArgs:  []string{"arg1=value1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(model.OktetoGitCommitEnvVar)
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Namespace: "test",
						IsOkteto:  tt.isOkteto,
					},
				},
				CurrentContext: "test",
			}
			os.Setenv(model.OktetoGitCommitEnvVar, tt.okGitCommitEnv)
			result := OptsFromManifest(tt.serviceName, tt.buildInfo, tt.initialOpts)
			assert.Equal(t, tt.expected, result)
		})
	}
}
