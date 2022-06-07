package build

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
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

func Test_OptsFromBuildInfo(t *testing.T) {
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
		name           string
		serviceName    string
		buildInfo      *model.BuildInfo
		okGitCommitEnv string
		isOkteto       bool
		initialOpts    *types.BuildOptions
		expected       *types.BuildOptions
	}{
		{
			name:        "empty-values-is-okteto",
			serviceName: "service",
			buildInfo:   &model.BuildInfo{},
			isOkteto:    true,
			expected: &types.BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:okteto",
				BuildArgs:  []string{},
				AutogenTag: true,
			},
		},
		{
			name:           "empty-values-is-okteto-local",
			serviceName:    "service",
			buildInfo:      &model.BuildInfo{},
			okGitCommitEnv: "dev1235466",
			isOkteto:       true,
			expected: &types.BuildOptions{
				Tag:        "okteto.dev/movies-service:okteto",
				OutputMode: oktetoLog.TTYFormat,
				BuildArgs:  []string{},
				AutogenTag: true,
			},
		},
		{
			name:           "empty-values-is-okteto-pipeline",
			serviceName:    "service",
			buildInfo:      &model.BuildInfo{},
			okGitCommitEnv: "1235466",
			isOkteto:       true,
			expected: &types.BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:114921fe985b5f874c8d312b0a098959da6d119209c9d1e42a89c4309569692d",
				BuildArgs:  []string{},
				AutogenTag: true,
			},
		},
		{
			name:        "empty-values-is-okteto-pipeline-withArgs",
			serviceName: "service",
			buildInfo: &model.BuildInfo{
				Args: model.Environment{
					{
						Name:  "arg1",
						Value: "value1",
					},
				}},
			okGitCommitEnv: "1235466",
			isOkteto:       true,
			expected: &types.BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:c0776074a88fa37835b1dfa67365b6a6b08b11c4cf49a9d42524ea9797959e58",
				BuildArgs:  []string{"arg1=value1"},
				AutogenTag: true,
			},
		},
		{
			name:        "empty-values-is-not-okteto",
			serviceName: "service",
			buildInfo:   &model.BuildInfo{},
			isOkteto:    false,
			expected: &types.BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				BuildArgs:  []string{},
			},
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
			initialOpts: &types.BuildOptions{
				OutputMode: "tty",
			},
			isOkteto: true,
			expected: &types.BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:okteto",
				File:       "CustomDockerfile",
				Target:     "build",
				Path:       "service",
				CacheFrom:  []string{"cache-image"},
				BuildArgs:  []string{"arg1=value1"},
				AutogenTag: true,
			},
		},
		{
			name:        "all-values-image",
			serviceName: "service",
			buildInfo: &model.BuildInfo{
				Image:      "okteto.dev/mycustomimage:dev",
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
			initialOpts: &types.BuildOptions{
				OutputMode: "tty",
			},
			isOkteto: true,
			expected: &types.BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/mycustomimage:dev",
				File:       "CustomDockerfile",
				Target:     "build",
				Path:       "service",
				CacheFrom:  []string{"cache-image"},
				BuildArgs:  []string{"arg1=value1"},
				AutogenTag: false,
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
			manifest := &model.Manifest{
				Name: "movies",
				Build: model.ManifestBuild{
					tt.serviceName: tt.buildInfo,
				},
			}
			os.Setenv(model.OktetoGitCommitEnvVar, tt.okGitCommitEnv)
			result := OptsFromBuildInfo(manifest.Name, tt.serviceName, manifest.Build[tt.serviceName], tt.initialOpts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOptsFromBuildInfo(t *testing.T) {
	buildName := "frontendTest"
	mockDir := "mockDir"

	originalWd, errwd := os.Getwd()
	if errwd != nil {
		t.Fatal(errwd)
	}

	dir := t.TempDir()
	log.Printf("created tempdir: %s", dir)

	os.Chdir(dir)
	defer os.Chdir(originalWd)

	err := os.Mkdir(buildName, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Mkdir(mockDir, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}

	contextPath := filepath.Join(dir, buildName)
	log.Printf("created context dir: %s", contextPath)

	tests := []struct {
		name               string
		svcName            string
		dockerfile         string
		fileExpected       string
		dockerfilesCreated []string
		expectedError      string
	}{
		{
			name:               "dockerfile is abs path",
			svcName:            "t1",
			dockerfile:         filepath.Join(contextPath, "Dockerfile"),
			fileExpected:       filepath.Join(contextPath, "Dockerfile"),
			dockerfilesCreated: nil,
			expectedError:      "",
		},
		{
			name:               "dockerfile is NOT relative to context",
			svcName:            "t2",
			dockerfile:         "Dockerfile",
			fileExpected:       "Dockerfile",
			dockerfilesCreated: []string{"Dockerfile"},
			expectedError:      fmt.Sprintf(warningDockerfilePath, "t2", "Dockerfile", buildName),
		},
		{
			name:               "dockerfile in root and dockerfile in context path",
			svcName:            "t3",
			dockerfile:         "Dockerfile",
			fileExpected:       filepath.Join(buildName, "Dockerfile"),
			dockerfilesCreated: []string{"Dockerfile", filepath.Join(buildName, "Dockerfile")},
			expectedError:      fmt.Sprintf(doubleDockerfileWarning, "t3", buildName, "Dockerfile"),
		},
		{
			name:               "dockerfile is relative to context",
			svcName:            "t4",
			dockerfile:         "Dockerfile",
			fileExpected:       filepath.Join(buildName, "Dockerfile"),
			dockerfilesCreated: []string{filepath.Join(buildName, "Dockerfile")},
			expectedError:      "",
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			if tt.dockerfilesCreated != nil {
				for _, df := range tt.dockerfilesCreated {
					defer removeFile(df)
					_, err := os.Create(df)
					if err != nil {
						t.Fatal(err)
					}

					log.Printf("created docker file: %s", df)
				}
			}

			var buf bytes.Buffer
			oktetoLog.SetOutput(&buf)

			defer func() {
				oktetoLog.SetOutput(os.Stderr)
			}()

			file := extractFromContextAndDockerfile(buildName, tt.dockerfile, tt.svcName)
			warningErr := strings.TrimSuffix(buf.String(), "\n")

			if warningErr != "" && tt.expectedError == "" {
				t.Fatalf("Got error but wasn't expecting any: %s", warningErr)
			}

			if warningErr == "" && tt.expectedError != "" {
				t.Fatal("error expected not thrown")
			}

			if warningErr != "" && tt.expectedError != "" && !strings.Contains(warningErr, tt.expectedError) {
				t.Fatalf("Error expected '%s', does not match error thrown: '%s'", tt.expectedError, warningErr)
			}

			assert.Equal(t, tt.fileExpected, file)

		})
	}
}

func removeFile(s string) error {
	// rm context and dockerfile
	err := os.Remove(s)
	if err != nil {
		return err
	}

	return nil
}
