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
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
				Registry:  "this.is.my.okteto.registry",
			},
		},
		CurrentContext: "test",
	}
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

func Test_getBuildOptionsFromManifest(t *testing.T) {
	tests := []struct {
		name              string
		service           string
		isOktetoContext   bool
		manifestBuildInfo *model.BuildInfo
		expected          *BuildOptions
		nameEnv           string
		commitEnv         string
	}{
		{
			name:            "no-okteto-context",
			service:         "service",
			isOktetoContext: false,
			manifestBuildInfo: &model.BuildInfo{
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
			expected: &BuildOptions{
				Path: ".",
				File: "Dockerfile",
			},
		},
		{
			name:            "okteto-context-no-image-name-env",
			service:         "service",
			nameEnv:         "parent",
			isOktetoContext: true,
			manifestBuildInfo: &model.BuildInfo{
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
			expected: &BuildOptions{
				Path: ".",
				File: "Dockerfile",
				Tag:  "okteto.dev/parent-service:okteto",
			},
		},
		{
			name:            "okteto-context-no-image-name",
			service:         "service",
			isOktetoContext: true,
			manifestBuildInfo: &model.BuildInfo{
				Context:    ".",
				Dockerfile: "Dockerfile",
				Name:       "parent",
			},
			expected: &BuildOptions{
				Path: ".",
				File: "Dockerfile",
				Tag:  "okteto.dev/parent-service:okteto",
			},
		},
		{
			name:            "manifest-no-image-no-name",
			service:         "service",
			isOktetoContext: true,
			manifestBuildInfo: &model.BuildInfo{
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
			expected: &BuildOptions{
				Path: ".",
				File: "Dockerfile",
				Tag:  "okteto.dev/service:okteto",
			},
		},
		{
			name:    "manifest-image",
			service: "service",
			manifestBuildInfo: &model.BuildInfo{
				Context:    ".",
				Dockerfile: "Dockerfile",
				Image:      "okteto.dev/myservice:mytag",
			},
			expected: &BuildOptions{
				Path: ".",
				File: "Dockerfile",
				Tag:  "okteto.dev/myservice:mytag",
			},
		},
		{
			name:            "is-pipeline",
			service:         "service",
			isOktetoContext: true,
			manifestBuildInfo: &model.BuildInfo{
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
			nameEnv:   "parent",
			commitEnv: "1234568555569985",
			expected: &BuildOptions{
				Path: ".",
				File: "Dockerfile",
				Tag:  "okteto.dev/parent-service:55440a5a4e502b6466f865d8f949c2dbc6651369ef7813099b7f24198aa62fd9",
			},
		},
		{
			name:            "is-not-pipeline",
			service:         "service",
			isOktetoContext: true,
			manifestBuildInfo: &model.BuildInfo{
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
			nameEnv:   "parent",
			commitEnv: "dev1234568555569985",
			expected: &BuildOptions{
				Path: ".",
				File: "Dockerfile",
				Tag:  "okteto.dev/parent-service:okteto",
			},
		},
		{
			name:            "is-volume-mounts-is-pipeline",
			service:         "service",
			isOktetoContext: true,
			manifestBuildInfo: &model.BuildInfo{
				Context:    ".",
				Dockerfile: "Dockerfile",
				VolumesToInclude: []model.StackVolume{
					{
						LocalPath:  "local",
						RemotePath: "remote",
					},
				},
			},
			nameEnv:   "parent",
			commitEnv: "1234568555569985",
			expected: &BuildOptions{
				Path: ".",
				File: "Dockerfile",
				Tag:  "okteto.dev/parent-service:okteto-with-volume-mounts",
			},
		},
		{
			name:            "is-volume-mounts-is-not-pipeline",
			service:         "service",
			isOktetoContext: true,
			manifestBuildInfo: &model.BuildInfo{
				Context:    ".",
				Dockerfile: "Dockerfile",
				VolumesToInclude: []model.StackVolume{
					{
						LocalPath:  "local",
						RemotePath: "remote",
					},
				},
			},
			nameEnv:   "parent",
			commitEnv: "dev1234568555569985",
			expected: &BuildOptions{
				Path: ".",
				File: "Dockerfile",
				Tag:  "okteto.dev/parent-service:okteto-with-volume-mounts",
			},
		},
		{
			name:            "default-values",
			service:         "service",
			isOktetoContext: true,
			manifestBuildInfo: &model.BuildInfo{
				Target:      "build",
				Context:     ".",
				Dockerfile:  "Dockerfile",
				CacheFrom:   []string{"cache-url"},
				ExportCache: "export-url",
			},
			nameEnv: "parent",
			expected: &BuildOptions{
				Path:        ".",
				File:        "Dockerfile",
				Tag:         "okteto.dev/parent-service:okteto",
				Target:      "build",
				CacheFrom:   []string{"cache-url"},
				ExportCache: "export-url",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(model.OktetoNameEnvVar, tt.nameEnv)
			os.Setenv(model.OktetoGitCommitEnvVar, tt.commitEnv)
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Namespace: "test",
						IsOkteto:  tt.isOktetoContext,
					},
				},
				CurrentContext: "test",
			}

			res := getBuildOptionsFromManifest(tt.service, tt.manifestBuildInfo)
			assert.Exactly(t, tt.expected, res)
		})
	}
}

func Test_overrideManifestBuildOptions(t *testing.T) {
	tests := []struct {
		name                 string
		isOktetoContext      bool
		manifestBuildOptions *BuildOptions
		cmdBuildOptions      *BuildOptions
		expected             *BuildOptions
	}{
		{
			name:                 "nil-options",
			manifestBuildOptions: nil,
			cmdBuildOptions:      nil,
			expected: &BuildOptions{
				OutputMode: "tty",
			},
		},
		{
			name:            "build-global",
			isOktetoContext: true,
			manifestBuildOptions: &BuildOptions{
				Tag: "okteto.dev/service:okteto",
			},
			cmdBuildOptions: &BuildOptions{
				BuildToGlobal: true,
			},
			expected: &BuildOptions{
				Tag:        "okteto.global/service:okteto",
				OutputMode: "tty",
			},
		},
		{
			name: "override-all-cmdOptions",
			manifestBuildOptions: &BuildOptions{
				Tag: "okteto.dev/service:okteto",
			},
			cmdBuildOptions: &BuildOptions{
				Tag:        "okteto.dev/service:newtag",
				NoCache:    true,
				Namespace:  "namespace",
				K8sContext: "context",
				Secrets:    []string{"SECRET=mysecret"},
				CacheFrom:  []string{"cache-url"},
				BuildArgs:  []string{"ARG1=argvalue"},
				OutputMode: "plain",
			},
			expected: &BuildOptions{
				Tag:        "okteto.dev/service:newtag",
				OutputMode: "plain",
				NoCache:    true,
				Namespace:  "namespace",
				K8sContext: "context",
				Secrets:    []string{"SECRET=mysecret"},
				CacheFrom:  []string{"cache-url"},
				BuildArgs:  []string{"ARG1=argvalue"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Namespace: "test",
						IsOkteto:  tt.isOktetoContext,
					},
				},
				CurrentContext: "test",
			}
			res := overrideManifestBuildOptions(tt.manifestBuildOptions, tt.cmdBuildOptions)
			assert.Exactly(t, tt.expected, res)
		})
	}
}

func Test_OptsFromManifest(t *testing.T) {
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
				Registry:  "this.is.my.okteto.registry",
			},
		},
		CurrentContext: "test",
	}
	tests := []struct {
		name              string
		service           string
		manifestBuildInfo *model.BuildInfo
		cmdBuildOptions   *BuildOptions

		okGitCommitEnv string
		isOkteto       bool
		expected       *BuildOptions
	}{
		{
			name:              "empty-values-is-okteto",
			service:           "service",
			manifestBuildInfo: &model.BuildInfo{Name: "movies"},
			isOkteto:          true,
			expected: &BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:okteto",
			},
		},
		{
			name:              "empty-values-is-okteto-local",
			service:           "service",
			manifestBuildInfo: &model.BuildInfo{Name: "movies"},
			okGitCommitEnv:    "dev1235466",
			isOkteto:          true,
			expected: &BuildOptions{
				Tag:        "okteto.dev/movies-service:okteto",
				OutputMode: oktetoLog.TTYFormat,
			},
		},
		{
			name:              "empty-values-is-okteto-pipeline",
			service:           "service",
			manifestBuildInfo: &model.BuildInfo{Name: "movies"},
			okGitCommitEnv:    "1235466",
			isOkteto:          true,
			expected: &BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:114921fe985b5f874c8d312b0a098959da6d119209c9d1e42a89c4309569692d",
			},
		},
		{
			name:    "empty-values-is-okteto-pipeline-withArgs",
			service: "service",
			manifestBuildInfo: &model.BuildInfo{
				Name: "movies",
				Args: model.Environment{
					{
						Name:  "arg1",
						Value: "value1",
					},
				}},
			okGitCommitEnv: "1235466",
			isOkteto:       true,
			expected: &BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:c0776074a88fa37835b1dfa67365b6a6b08b11c4cf49a9d42524ea9797959e58",
				BuildArgs:  []string{"arg1=value1"},
			},
		},
		{
			name:              "empty-values-is-not-okteto",
			service:           "service",
			manifestBuildInfo: &model.BuildInfo{Name: "movies"},
			isOkteto:          false,
			expected: &BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
			},
		},
		{
			name:    "all-values-no-image",
			service: "service",
			manifestBuildInfo: &model.BuildInfo{
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
			cmdBuildOptions: &BuildOptions{
				OutputMode: "tty",
			},
			isOkteto: true,
			expected: &BuildOptions{
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
			os.Setenv(model.OktetoGitCommitEnvVar, tt.okGitCommitEnv)
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Namespace: "test",
						IsOkteto:  tt.isOkteto,
					},
				},
				CurrentContext: "test",
			}
			result := OptsFromManifest(tt.service, tt.manifestBuildInfo, tt.cmdBuildOptions)
			assert.Equal(t, tt.expected, result)
		})
	}
}
