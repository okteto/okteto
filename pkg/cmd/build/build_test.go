package build

import (
	"os"
	"reflect"
	"testing"

	okErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
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
			want:  okErrors.UserError{},
		},
		{
			name:  "okteto-global-valid",
			image: "okteto.global/image",
			want:  nil,
		},
		{
			name:  "okteto-global-not-valid",
			image: "okteto.global/image/hello",
			want:  okErrors.UserError{},
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

func Test_setOktetoImageTag(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		okGitCommitEnv string
		expected       string
	}{
		{
			name:     "okteto-dev-with-no-commit",
			input:    "service",
			expected: "okteto.dev/service:dev",
		},
		{
			name:           "okteto-dev-with-commit",
			input:          "service",
			okGitCommitEnv: "tag",
			expected:       "okteto.dev/service:tag",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("OKTETO_GIT_COMMIT", tt.okGitCommitEnv)
			if result := setOktetoImageTag(tt.input); result != tt.expected {
				t.Errorf("setOktetoImageTag = %v, want %v", result, tt.expected)
			}
			os.Unsetenv("OKTETO_GIT_COMMIT")
		})
	}
}

func Test_OptsFromManifest(t *testing.T) {
	tests := []struct {
		name           string
		serviceName    string
		buildInfo      *model.BuildInfo
		okGitCommitEnv string
		expected       BuildOptions
	}{
		{
			name:        "empty-values",
			serviceName: "service",
			buildInfo:   &model.BuildInfo{},
			expected: BuildOptions{
				Tag: "okteto.dev/service:dev",
			},
		},
		{
			name:        "all-values-no-commit-env-no-image",
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
			expected: BuildOptions{
				Tag:       "okteto.dev/service:dev",
				File:      "CustomDockerfile",
				Target:    "build",
				Path:      "service",
				CacheFrom: []string{"cache-image"},
				BuildArgs: []string{"arg1=value1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("OKTETO_GIT_COMMIT", tt.okGitCommitEnv)
			result := OptsFromManifest(tt.serviceName, tt.buildInfo)
			assert.Equal(t, tt.expected, result)
			os.Unsetenv("OKTETO_GIT_COMMIT")
		})
	}
}
