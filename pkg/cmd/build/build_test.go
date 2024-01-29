// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	"github.com/okteto/okteto/pkg/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_validateImage(t *testing.T) {
	okCtx := &okteto.ContextStateless{
		Store: &okteto.ContextStore{
			Contexts: map[string]*okteto.Context{
				"test": {
					Namespace: "test",
					IsOkteto:  true,
					Registry:  "test",
				},
			},
			CurrentContext: "test",
		},
	}
	tests := []struct {
		want  error
		name  string
		image string
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
			if got := validateImage(okCtx, tt.image); reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("build.validateImage = %v, want %v", reflect.TypeOf(got), reflect.TypeOf(tt.want))
			}
		})
	}
}

type mockRegistry struct {
	registry         string
	repo             string
	tag              string
	isGlobal         bool
	isOktetoRegistry bool
}

func (*mockRegistry) HasGlobalPushAccess() (bool, error) {
	return false, nil
}

func (mr *mockRegistry) IsOktetoRegistry(image string) bool {
	return mr.isOktetoRegistry
}

func (mr *mockRegistry) IsGlobalRegistry(image string) bool {
	return mr.isGlobal
}

func (mr *mockRegistry) GetRegistryAndRepo(_ string) (string, string) {
	return mr.registry, mr.repo
}

func (mr *mockRegistry) GetRepoNameAndTag(_ string) (string, string) {
	return mr.repo, mr.tag
}

func Test_OptsFromBuildInfo(t *testing.T) {
	context := okteto.Context{
		Namespace: "test",
		Registry:  "registry.okteto",
	}

	namespaceEnvVar := build.Arg{
		Name: model.OktetoNamespaceEnvVar, Value: context.Namespace,
	}

	serviceContext := "service"
	serviceDockerfile := "CustomDockerfile"

	originalWd, errwd := os.Getwd()
	if errwd != nil {
		t.Fatal(errwd)
	}

	dir := t.TempDir()

	require.NoError(t, os.Chdir(dir))
	defer func() {
		require.NoError(t, os.Chdir(originalWd))
	}()
	require.NoError(t, os.Mkdir(serviceContext, os.ModePerm))

	df := filepath.Join(serviceContext, serviceDockerfile)
	dockerfile, errCreate := os.Create(df)
	require.NoError(t, errCreate)

	defer func() {
		require.NoError(t, dockerfile.Close())
		require.NoError(t, removeFile(df))
	}()

	tests := []struct {
		buildInfo   *build.Info
		initialOpts *types.BuildOptions
		expected    *types.BuildOptions
		name        string
		serviceName string
		mr          mockRegistry
		isOkteto    bool
	}{
		{
			name:        "is-okteto-empty-buildInfo",
			serviceName: "service",
			buildInfo:   &build.Info{},
			mr: mockRegistry{
				isOktetoRegistry: true,
				registry:         "okteto.dev",
				repo:             "movies-service",
			},
			isOkteto: true,
			expected: &types.BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:okteto",
				BuildArgs:  []string{namespaceEnvVar.String()},
			},
		},
		{
			name:        "not-okteto-empty-buildInfo",
			serviceName: "service",
			buildInfo:   &build.Info{},
			isOkteto:    false,
			expected: &types.BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				BuildArgs:  []string{},
			},
		},
		{
			name:        "is-okteto-missing-image-buildInfo",
			serviceName: "service",
			buildInfo: &build.Info{
				Context:    serviceContext,
				Dockerfile: serviceDockerfile,
				Target:     "build",
				CacheFrom:  []string{"cache-image"},
				Args: build.Args{
					{
						Name:  "arg1",
						Value: "value1",
					},
				},
			},
			initialOpts: &types.BuildOptions{
				OutputMode: "tty",
			},
			mr: mockRegistry{
				isOktetoRegistry: true,
				registry:         "okteto.dev",
				repo:             "movies-service",
			},
			isOkteto: true,
			expected: &types.BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:okteto",
				File:       filepath.Join(serviceContext, serviceDockerfile),
				Target:     "build",
				Path:       "service",
				CacheFrom:  []string{"cache-image"},
				BuildArgs:  []string{namespaceEnvVar.String(), "arg1=value1"},
			},
		},
		{
			name:        "is-okteto-missing-image-buildInfo-with-volumes",
			serviceName: "service",
			buildInfo: &build.Info{
				Context:    serviceContext,
				Dockerfile: serviceDockerfile,
				Target:     "build",
				CacheFrom:  []string{"cache-image"},
				Args: build.Args{
					{
						Name:  "arg1",
						Value: "value1",
					},
				},
				VolumesToInclude: []build.VolumeMounts{
					{
						LocalPath:  "a",
						RemotePath: "b",
					},
				},
			},
			mr: mockRegistry{
				isOktetoRegistry: true,
				registry:         "okteto.dev",
				repo:             "movies-service",
			},
			initialOpts: &types.BuildOptions{
				OutputMode: "tty",
			},
			isOkteto: true,
			expected: &types.BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:okteto-with-volume-mounts",
				File:       filepath.Join(serviceContext, serviceDockerfile),
				Target:     "build",
				Path:       "service",
				CacheFrom: []string{
					"cache-image",
				},
				BuildArgs: []string{namespaceEnvVar.String(), "arg1=value1"},
			},
		},
		{
			name:        "is-okteto-has-image-buildInfo",
			serviceName: "service",
			buildInfo: &build.Info{
				Image:      "okteto.dev/mycustomimage:dev",
				Context:    serviceContext,
				Dockerfile: serviceDockerfile,
				Target:     "build",
				CacheFrom:  []string{"cache-image"},
				Args: build.Args{
					namespaceEnvVar,
					{
						Name:  "arg1",
						Value: "value1",
					},
				},
				Secrets: map[string]string{
					"mysecret": "source",
				},
				ExportCache: []string{"export-image"},
			},
			mr: mockRegistry{
				isOktetoRegistry: true,
				registry:         "okteto.dev",
				repo:             "mycustomimage",
			},
			initialOpts: &types.BuildOptions{
				OutputMode: "tty",
			},
			isOkteto: true,
			expected: &types.BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/mycustomimage:dev",
				File:       filepath.Join(serviceContext, serviceDockerfile),
				Target:     "build",
				Path:       "service",
				CacheFrom: []string{
					"cache-image",
				},
				BuildArgs: []string{namespaceEnvVar.String(), "arg1=value1"},
				Secrets:   []string{"id=mysecret,src=source"},
				ExportCache: []string{
					"export-image",
				},
			},
		},
		{
			name:        "has-platform-option",
			serviceName: "service",
			buildInfo:   &build.Info{},
			initialOpts: &types.BuildOptions{
				Platform: "linux/amd64"},
			isOkteto: true,
			mr: mockRegistry{
				isOktetoRegistry: true,
				registry:         "okteto.dev",
				repo:             "movies-service",
			},
			expected: &types.BuildOptions{
				BuildArgs:  []string{namespaceEnvVar.String()},
				Platform:   "linux/amd64",
				Tag:        "okteto.dev/movies-service:okteto",
				OutputMode: "tty",
			},
		},
		{
			name:        "has-platform-option",
			serviceName: "service",
			buildInfo:   &build.Info{},
			initialOpts: &types.BuildOptions{
				BuildArgs: []string{
					"arg1=value1",
				},
			},
			isOkteto: true,
			mr: mockRegistry{
				isOktetoRegistry: true,
				registry:         "okteto.dev",
				repo:             "movies-service",
			},
			expected: &types.BuildOptions{
				BuildArgs: []string{
					namespaceEnvVar.String(),
					"arg1=value1",
				},
				Tag:        "okteto.dev/movies-service:okteto",
				OutputMode: "tty",
			},
		},
		{
			name:        "only key",
			serviceName: "service",
			buildInfo: &build.Info{
				Args: build.Args{
					{
						Name:  "arg1",
						Value: "value2",
					},
				},
			},
			initialOpts: &types.BuildOptions{
				BuildArgs: []string{
					"arg1",
				},
			},
			isOkteto: true,
			mr: mockRegistry{
				isOktetoRegistry: true,
				registry:         "okteto.dev",
				repo:             "movies-service",
			},
			expected: &types.BuildOptions{
				BuildArgs: []string{
					namespaceEnvVar.String(),
					"arg1=",
				},
				Tag:        "okteto.dev/movies-service:okteto",
				OutputMode: "tty",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okCtx := &okteto.ContextStateless{
				Store: &okteto.ContextStore{
					Contexts: map[string]*okteto.Context{
						"test": {
							Namespace: "test",
							IsOkteto:  tt.isOkteto,
						},
					},
					CurrentContext: "test",
				},
			}
			manifest := &model.Manifest{
				Name: "movies",
				Build: build.ManifestBuild{
					tt.serviceName: tt.buildInfo,
				},
			}

			result := OptsFromBuildInfo(manifest.Name, tt.serviceName, manifest.Build[tt.serviceName], tt.initialOpts, &tt.mr, okCtx)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestOptsFromBuildInfoForRemoteDeploy(t *testing.T) {
	tests := []struct {
		buildInfo *build.Info
		expected  *types.BuildOptions
		name      string
	}{
		{
			name: "all fields set",
			buildInfo: &build.Info{
				Name:        "movies-service",
				Context:     "service",
				Dockerfile:  "Dockerfile",
				Target:      "build",
				CacheFrom:   []string{"cache-image"},
				Image:       "okteto.dev/movies-service:dev",
				ExportCache: []string{"export-image"},
			},
			expected: &types.BuildOptions{
				File:       "Dockerfile",
				OutputMode: "deploy",
				Path:       "service",
			},
		},
		{
			name: "just the fields needed",
			buildInfo: &build.Info{
				Name:        "movies-service",
				Context:     "service",
				Dockerfile:  "Dockerfile",
				Target:      "build",
				CacheFrom:   []string{"cache-image"},
				Image:       "okteto.dev/movies-service:dev",
				ExportCache: []string{"export-image"},
			},
			expected: &types.BuildOptions{
				File:       "Dockerfile",
				OutputMode: "deploy",
				Path:       "service",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := OptsFromBuildInfoForRemoteDeploy(tt.buildInfo, &types.BuildOptions{OutputMode: "deploy"})
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractFromContextAndDockerfile(t *testing.T) {
	buildName := "frontendTest"
	mockDir := "mockDir"

	originalWd, errwd := os.Getwd()
	require.NoError(t, errwd)

	dir := t.TempDir()

	require.NoError(t, os.Chdir(dir))
	defer func() {
		require.NoError(t, os.Chdir(originalWd))
	}()

	require.NoError(t, os.Mkdir(buildName, os.ModePerm))

	require.NoError(t, os.Mkdir(mockDir, os.ModePerm))

	contextPath := filepath.Join(dir, buildName)
	log.Printf("created context dir: %s", contextPath)

	tests := []struct {
		name               string
		svcName            string
		dockerfile         string
		fileExpected       string
		optionalContext    string
		expectedError      string
		dockerfilesCreated []string
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
		{
			name:               "one dockerfile in root no warning",
			svcName:            "t5",
			dockerfile:         "Dockerfile",
			fileExpected:       "Dockerfile",
			optionalContext:    ".",
			dockerfilesCreated: []string{"Dockerfile"},
			expectedError:      "",
		},
		{
			name:               "dockerfile in root, not showing 2 dockerfiles warning",
			svcName:            "t6",
			dockerfile:         "./Dockerfile",
			fileExpected:       "Dockerfile",
			optionalContext:    ".",
			dockerfilesCreated: []string{"Dockerfile"},
			expectedError:      "",
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			if tt.dockerfilesCreated != nil {
				for _, df := range tt.dockerfilesCreated {
					df := df
					dfFile, err := os.Create(df)
					require.NoError(t, err)

					log.Printf("created docker file: %s", df)

					require.NoError(t, dfFile.Close())
					defer func() {
						require.NoError(t, removeFile(df))
					}()
				}
			}

			var buf bytes.Buffer
			oktetoLog.SetOutput(&buf)

			defer func() {
				oktetoLog.SetOutput(os.Stderr)
			}()

			contextTest := buildName
			if tt.optionalContext != "" {
				contextTest = tt.optionalContext
			}

			file := extractFromContextAndDockerfile(contextTest, tt.dockerfile, tt.svcName)
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

			require.Equal(t, tt.fileExpected, file)

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

func Test_replaceSecretsSourceEnvWithTempFile(t *testing.T) {
	t.Parallel()
	fakeFs := afero.NewMemMapFs()
	localSrcFile, err := afero.TempFile(fakeFs, t.TempDir(), "")
	require.NoError(t, err)

	tests := []struct {
		fs                      afero.Fs
		buildOptions            *types.BuildOptions
		name                    string
		secretTempFolder        string
		expectedErr             bool
		expectedReplacedSecrets bool
	}{
		{
			name:             "valid secret format",
			fs:               fakeFs,
			secretTempFolder: t.TempDir(),
			buildOptions: &types.BuildOptions{
				Secrets: []string{fmt.Sprintf("id=mysecret,src=%s", localSrcFile.Name())},
			},
			expectedErr:             false,
			expectedReplacedSecrets: true,
		},
		{
			name:             "valid secret format, reorder the fields",
			fs:               fakeFs,
			secretTempFolder: t.TempDir(),
			buildOptions: &types.BuildOptions{
				Secrets: []string{fmt.Sprintf("src=%s,id=mysecret", localSrcFile.Name())},
			},
			expectedErr:             false,
			expectedReplacedSecrets: true,
		},
		{
			name:             "valid secret format, only id",
			fs:               fakeFs,
			secretTempFolder: t.TempDir(),
			buildOptions: &types.BuildOptions{
				Secrets: []string{"id=mysecret"},
			},
			expectedErr: false,
		},
		{
			name:             "valid secret format, only source",
			fs:               fakeFs,
			secretTempFolder: t.TempDir(),
			buildOptions: &types.BuildOptions{
				Secrets: []string{fmt.Sprintf("src=%s", localSrcFile.Name())},
			},
			expectedErr:             false,
			expectedReplacedSecrets: true,
		},
		{
			name:             "invalid secret, local file does not exist",
			fs:               fakeFs,
			secretTempFolder: t.TempDir(),
			buildOptions: &types.BuildOptions{
				Secrets: []string{"id=mysecret,src=/file/invalid"},
			},
			expectedErr:             true,
			expectedReplacedSecrets: false,
		},
		{
			name:             "invalid secret, no = found",
			fs:               fakeFs,
			secretTempFolder: t.TempDir(),
			buildOptions: &types.BuildOptions{
				Secrets: []string{"mysecret"},
			},
			expectedErr:             true,
			expectedReplacedSecrets: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			initialSecrets := make([]string, len(tt.buildOptions.Secrets))
			copy(initialSecrets, tt.buildOptions.Secrets)
			err := replaceSecretsSourceEnvWithTempFile(tt.fs, tt.secretTempFolder, tt.buildOptions)
			require.Truef(t, tt.expectedErr == (err != nil), "not expected error")

			if tt.expectedReplacedSecrets {
				require.NotEqualValues(t, initialSecrets, tt.buildOptions.Secrets)
			} else {
				require.EqualValues(t, initialSecrets, tt.buildOptions.Secrets)
			}
		})
	}
}

func Test_createTempFileWithExpandedEnvsAtSource(t *testing.T) {
	fakeFs := afero.NewMemMapFs()

	localSrcFile, err := afero.TempFile(fakeFs, t.TempDir(), "")
	require.NoError(t, err)

	_, err = localSrcFile.Write([]byte(`localEnv: ${ENV_IN_FILE}`))
	require.NoError(t, err)

	tests := []struct {
		name        string
		fakeFs      afero.Fs
		sourceFile  string
		envValue    string
		expectedErr bool
	}{
		{
			name:       "no error",
			envValue:   "value of env",
			sourceFile: localSrcFile.Name(),
			fakeFs:     fakeFs,
		},
		{
			name:        "error - local file not exist",
			sourceFile:  "file",
			fakeFs:      fakeFs,
			expectedErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ENV_IN_FILE", tt.envValue)

			file, err := createTempFileWithExpandedEnvsAtSource(tt.fakeFs, tt.sourceFile, t.TempDir())
			if tt.expectedErr {
				require.Error(t, err)
				return
			}

			f, err := afero.ReadFile(tt.fakeFs, file)
			if !tt.expectedErr {
				require.NoError(t, err)
			}

			require.Contains(t, string(f), "value of env")
		})
	}
}

func Test_translateDockerErr(t *testing.T) {
	tests := []struct {
		input       error
		expectedErr error
		name        string
	}{
		{
			name:        "err is nil",
			input:       nil,
			expectedErr: nil,
		},
		{
			name:        "err is docker error",
			input:       fmt.Errorf("failed to dial gRPC: cannot connect to the Docker daemon"),
			expectedErr: errDockerDaemonConnection,
		},
		{
			name:        "err is not docker error",
			input:       assert.AnError,
			expectedErr: assert.AnError,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			got := translateDockerErr(tt.input)
			require.Equal(t, tt.expectedErr, got)
		})

	}
}

func Test_setOutputMode(t *testing.T) {
	tests := []struct {
		name                     string
		input                    string
		envBuildkitProgressValue string
		expected                 string
	}{
		{
			name:     "not empty input",
			input:    "outputmode",
			expected: "outputmode",
		},
		{
			name:     "empty input and empty env BUILDKIT_PROGRESS  - default output",
			input:    "",
			expected: "tty",
		},
		{
			name:                     "empty input and unknown env BUILDKIT_PROGRESS  - default output",
			input:                    "",
			envBuildkitProgressValue: "unknown",
			expected:                 "tty",
		},
		{
			name:                     "empty input and plain env BUILDKIT_PROGRESS  - default output",
			input:                    "",
			envBuildkitProgressValue: "plain",
			expected:                 "plain",
		},
		{
			name:                     "empty input and json env BUILDKIT_PROGRESS  - default output",
			input:                    "",
			envBuildkitProgressValue: "json",
			expected:                 "plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BUILDKIT_PROGRESS", tt.envBuildkitProgressValue)
			got := setOutputMode(tt.input)
			require.Equal(t, tt.expected, got)
		})
	}
}
