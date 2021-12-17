package build

import (
	"os"
	"reflect"
	"testing"

	okErrors "github.com/okteto/okteto/pkg/errors"
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

func Test_optsFromManifest(t *testing.T) {
	tests := []struct {
		name           string
		serviceName    string
		buildInfo      *model.BuildInfo
		okGitCommitEnv string
		isOkteto       bool
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
			isOkteto: true,
			expected: BuildOptions{
				Tag:       "okteto.dev/service:dev",
				File:      "service/CustomDockerfile",
				Target:    "build",
				Path:      "service",
				CacheFrom: []string{"cache-image"},
				BuildArgs: []string{"arg1=value1"},
			},
		},
		{
			name:        "all-values-no-commit-env-no-image-not-okteto",
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
			isOkteto: false,
			expected: BuildOptions{
				File:      "service/CustomDockerfile",
				Target:    "build",
				Path:      "service",
				CacheFrom: []string{"cache-image"},
				BuildArgs: []string{"arg1=value1"},
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
			os.Setenv("OKTETO_GIT_COMMIT", tt.okGitCommitEnv)
			result := optsFromManifest(tt.serviceName, tt.buildInfo)
			assert.Equal(t, tt.expected, result)
			os.Unsetenv("OKTETO_GIT_COMMIT")
		})
	}
}

func Test_BuildOptionsFromManifest(t *testing.T) {
	tests := []struct {
		name           string
		inputOptions   BuildOptions
		args           []string
		manifestYAML   []byte
		expectedError  bool
		existsManifest bool
		expectOptsLen  int
	}{
		{
			name:           "empty-manifest-path",
			inputOptions:   BuildOptions{},
			expectedError:  false,
			existsManifest: false,
			expectOptsLen:  0,
		},
		{
			name:         "manifest-path-no-args",
			inputOptions: BuildOptions{},
			manifestYAML: []byte(`
build:
  service1:
    context: service1
  service2:
    context: service2`),
			expectedError:  false,
			existsManifest: true,
			expectOptsLen:  2,
		},
		{
			name:         "manifest-path-args",
			inputOptions: BuildOptions{},
			args:         []string{"service2"},
			manifestYAML: []byte(`
build:
  service1:
    context: service1
  service2:
    context: service2`),
			expectedError:  false,
			existsManifest: true,
			expectOptsLen:  1,
		},
		{
			name:         "manifest-path-args-invalid",
			inputOptions: BuildOptions{},
			args:         []string{"service3"},
			manifestYAML: []byte(`
build:
  service1:
    context: service1
  service2:
    context: service2`),
			expectedError:  true,
			existsManifest: true,
			expectOptsLen:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := os.CreateTemp("", "okteto.yml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(file.Name())

			if _, err = file.Write(tt.manifestYAML); err != nil {
				t.Fatal("Failed to write to temporary file", err)
			}

			manifestPath := ""
			if tt.existsManifest {
				manifestPath = file.Name()
			}

			opts, err := BuildOptionsFromManifest(tt.inputOptions, tt.args, manifestPath)

			if len(opts) != tt.expectOptsLen {
				t.Errorf("Expected %v, got %v", tt.expectOptsLen, len(opts))
			}

			if tt.expectedError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
