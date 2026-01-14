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
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/config"
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
					Namespace:       "test",
					IsOkteto:        true,
					Registry:        "test",
					GlobalNamespace: "okteto",
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
			result := validateImages(okCtx, tt.image)
			assert.IsType(t, tt.want, result)
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
	dir, err := os.Getwd()
	require.NoError(t, err)
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
				Manifest: &model.Manifest{
					Name: "movies",
					Build: build.ManifestBuild{
						"service": {
							Image: "okteto.dev/movies-service:okteto",
						},
					},
				},
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
				Manifest: &model.Manifest{
					Name: "movies",
					Build: build.ManifestBuild{
						"service": {},
					},
				},
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
				Manifest: &model.Manifest{
					Name: "movies",
					Build: build.ManifestBuild{
						"service": {
							Context:    serviceContext,
							Dockerfile: serviceDockerfile,
							Image:      "okteto.dev/movies-service:okteto",
							Target:     "build",
							CacheFrom:  []string{"cache-image"},
							Args: build.Args{
								{
									Name:  "arg1",
									Value: "value1",
								},
							},
						},
					},
				},
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:okteto",
				File:       filepath.Join(dir, serviceContext, serviceDockerfile),
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
				Manifest: &model.Manifest{
					Name: "movies",
					Build: build.ManifestBuild{
						"service": {
							Context:    serviceContext,
							Image:      "okteto.dev/movies-service:okteto",
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
					},
				},
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:okteto",
				File:       filepath.Join(dir, serviceContext, serviceDockerfile),
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
				Manifest: &model.Manifest{
					Name: "movies",
					Build: build.ManifestBuild{
						"service": {
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
					},
				},
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/mycustomimage:dev",
				File:       filepath.Join(dir, serviceContext, serviceDockerfile),
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
				Manifest: &model.Manifest{
					Name: "movies",
					Build: build.ManifestBuild{
						"service": {
							Image: "okteto.dev/movies-service:okteto",
						},
					},
				},
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
				Manifest: &model.Manifest{
					Name: "movies",
					Build: build.ManifestBuild{
						"service": {
							Image: "okteto.dev/movies-service:okteto",
						},
					},
				},
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
				Manifest: &model.Manifest{
					Name: "movies",
					Build: build.ManifestBuild{
						"service": {
							Image: "okteto.dev/movies-service:okteto",
							Args: build.Args{
								{
									Name:  "arg1",
									Value: "value2",
								},
							},
						},
					},
				},
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

			result := OptsFromBuildInfo(manifest, tt.serviceName, manifest.Build[tt.serviceName], tt.initialOpts, &tt.mr, okCtx)
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
				Context:     "service",
				Dockerfile:  "Dockerfile",
				Target:      "build",
				CacheFrom:   []string{"cache-image"},
				Image:       "okteto.dev/movies-service:dev",
				ExportCache: []string{"export-image"},
			},
			expected: &types.BuildOptions{
				File:       "Dockerfile",
				OutputMode: DeployOutputModeOnBuild,
				Path:       "service",
			},
		},
		{
			name: "just the fields needed",
			buildInfo: &build.Info{
				Context:     "service",
				Dockerfile:  "Dockerfile",
				Target:      "build",
				CacheFrom:   []string{"cache-image"},
				Image:       "okteto.dev/movies-service:dev",
				ExportCache: []string{"export-image"},
			},
			expected: &types.BuildOptions{
				File:       "Dockerfile",
				OutputMode: DeployOutputModeOnBuild,
				Path:       "service",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := OptsFromBuildInfoForRemoteDeploy(tt.buildInfo, &types.BuildOptions{OutputMode: DeployOutputModeOnBuild})
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
		getWd              func() (string, error)
		dockerfilesCreated []string
	}{
		{
			name:       "dockerfile is abs path",
			svcName:    "t1",
			dockerfile: filepath.Join(contextPath, "Dockerfile"),
			getWd: func() (string, error) {
				return filepath.Clean("/"), nil
			},
			fileExpected:       filepath.Clean(filepath.Join(contextPath, "Dockerfile")),
			dockerfilesCreated: nil,
			expectedError:      "",
		},
		{
			name:       "dockerfile is NOT relative to context",
			svcName:    "t2",
			dockerfile: "Dockerfile",
			getWd: func() (string, error) {
				return filepath.Clean("/"), nil
			},
			fileExpected:       filepath.Clean("Dockerfile"),
			dockerfilesCreated: []string{"Dockerfile"},
			expectedError:      fmt.Sprintf(warningDockerfilePath, "t2", "Dockerfile", buildName),
		},
		{
			name:       "dockerfile in root and dockerfile in context path",
			svcName:    "t3",
			dockerfile: "Dockerfile",
			getWd: func() (string, error) {
				return filepath.Clean("/"), nil
			},
			fileExpected:       filepath.Clean(filepath.Join(filepath.Clean("/"), buildName, "Dockerfile")),
			dockerfilesCreated: []string{"Dockerfile", filepath.Join(buildName, "Dockerfile")},
			expectedError:      fmt.Sprintf(doubleDockerfileWarning, "t3", buildName, "Dockerfile"),
		},
		{
			name:       "dockerfile is relative to context",
			svcName:    "t4",
			dockerfile: "Dockerfile",
			getWd: func() (string, error) {
				return filepath.Clean("/"), nil
			},
			fileExpected:       filepath.Clean(filepath.Join(filepath.Clean("/"), buildName, "Dockerfile")),
			dockerfilesCreated: []string{filepath.Join(buildName, "Dockerfile")},
			expectedError:      "",
		},
		{
			name:            "one dockerfile in root no warning",
			svcName:         "t5",
			dockerfile:      "Dockerfile",
			fileExpected:    filepath.Clean("/Dockerfile"),
			optionalContext: ".",
			getWd: func() (string, error) {
				return filepath.Clean("/"), nil
			},
			dockerfilesCreated: []string{"Dockerfile"},
			expectedError:      "",
		},
		{
			name:            "dockerfile in root, not showing 2 dockerfiles warning",
			svcName:         "t6",
			dockerfile:      "./Dockerfile",
			fileExpected:    filepath.Clean("/Dockerfile"),
			optionalContext: ".",
			getWd: func() (string, error) {
				return filepath.Clean("/"), nil
			},
			dockerfilesCreated: []string{"Dockerfile"},
			expectedError:      "",
		},
		{
			name:            "dockerfile in folder, not showing 2 dockerfiles warning",
			svcName:         "t7",
			dockerfile:      "Dockerfile",
			fileExpected:    filepath.Clean("/Dockerfile"),
			optionalContext: ".",
			getWd: func() (string, error) {
				return filepath.Clean("/"), nil
			},
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

			file := extractFromContextAndDockerfile(contextTest, tt.dockerfile, tt.svcName, tt.getWd)
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

func Test_createSecretTempFolder(t *testing.T) {
	// Set up a temporary okteto home for testing
	tempDir := t.TempDir()
	t.Setenv("OKTETO_HOME", tempDir)

	t.Run("creates UUID subfolder", func(t *testing.T) {
		folder1, err := createSecretTempFolder()
		require.NoError(t, err)
		require.NotEmpty(t, folder1)

		// Verify folder exists
		info, err := os.Stat(folder1)
		require.NoError(t, err)
		require.True(t, info.IsDir())

		// Verify it's under .secret/
		baseSecretFolder := filepath.Join(config.GetOktetoHome(), ".secret")
		require.Contains(t, folder1, baseSecretFolder)

		// Verify the subfolder name is a valid UUID
		folderName := filepath.Base(folder1)
		_, err = uuid.Parse(folderName)
		require.NoError(t, err, "folder name should be a valid UUID")
	})

	t.Run("creates unique folders for multiple calls", func(t *testing.T) {
		folder1, err := createSecretTempFolder()
		require.NoError(t, err)

		folder2, err := createSecretTempFolder()
		require.NoError(t, err)

		// Verify they are different
		require.NotEqual(t, folder1, folder2, "each call should create a unique folder")

		// Verify both folders exist
		_, err = os.Stat(folder1)
		require.NoError(t, err)
		_, err = os.Stat(folder2)
		require.NoError(t, err)
	})

	t.Run("has correct permissions", func(t *testing.T) {
		folder, err := createSecretTempFolder()
		require.NoError(t, err)

		info, err := os.Stat(folder)
		require.NoError(t, err)

		// Verify permissions are 0700 (owner only)
		require.Equal(t, os.FileMode(0700), info.Mode().Perm())
	})
}

func Test_cleanupOldSecretFolders(t *testing.T) {
	// Set up a temporary okteto home for testing
	tempDir := t.TempDir()
	t.Setenv("OKTETO_HOME", tempDir)

	baseSecretFolder := filepath.Join(config.GetOktetoHome(), ".secret")
	require.NoError(t, os.MkdirAll(baseSecretFolder, 0700))

	t.Run("removes folders older than 24 hours", func(t *testing.T) {
		// Create an old UUID folder
		oldUUID := uuid.New().String()
		oldFolder := filepath.Join(baseSecretFolder, oldUUID)
		require.NoError(t, os.MkdirAll(oldFolder, 0700))

		// Make it old by changing modification time
		oldTime := time.Now().Add(-25 * time.Hour)
		require.NoError(t, os.Chtimes(oldFolder, oldTime, oldTime))

		// Verify folder exists before cleanup
		_, err := os.Stat(oldFolder)
		require.NoError(t, err)

		// Run cleanup
		cleanupOldSecretFolders()

		// Verify folder was removed
		_, err = os.Stat(oldFolder)
		require.True(t, os.IsNotExist(err), "old folder should be removed")
	})

	t.Run("keeps folders newer than 24 hours", func(t *testing.T) {
		// Create a recent UUID folder
		recentUUID := uuid.New().String()
		recentFolder := filepath.Join(baseSecretFolder, recentUUID)
		require.NoError(t, os.MkdirAll(recentFolder, 0700))

		// Verify folder exists before cleanup
		_, err := os.Stat(recentFolder)
		require.NoError(t, err)

		// Run cleanup
		cleanupOldSecretFolders()

		// Verify folder still exists
		_, err = os.Stat(recentFolder)
		require.NoError(t, err, "recent folder should not be removed")
	})

	t.Run("ignores non-UUID folders", func(t *testing.T) {
		// Create a non-UUID folder
		nonUUIDFolder := filepath.Join(baseSecretFolder, "not-a-uuid")
		require.NoError(t, os.MkdirAll(nonUUIDFolder, 0700))

		// Make it old
		oldTime := time.Now().Add(-25 * time.Hour)
		require.NoError(t, os.Chtimes(nonUUIDFolder, oldTime, oldTime))

		// Run cleanup
		cleanupOldSecretFolders()

		// Verify non-UUID folder still exists (not removed)
		_, err := os.Stat(nonUUIDFolder)
		require.NoError(t, err, "non-UUID folder should not be removed")
	})

	t.Run("ignores files in base folder", func(t *testing.T) {
		// Create a file in the base folder
		testFile := filepath.Join(baseSecretFolder, "test-file.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0600))

		// Run cleanup (should not panic or error)
		cleanupOldSecretFolders()

		// Verify file still exists
		_, err := os.Stat(testFile)
		require.NoError(t, err, "file should not be removed")
	})

	t.Run("handles missing base folder gracefully", func(t *testing.T) {
		// Use a different temp dir where .secret doesn't exist
		tempDir2 := t.TempDir()
		t.Setenv("OKTETO_HOME", tempDir2)

		// Should not panic
		require.NotPanics(t, func() {
			cleanupOldSecretFolders()
		})
	})
}
