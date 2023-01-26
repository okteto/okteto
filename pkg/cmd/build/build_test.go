package build

import (
	"bufio"
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
	context := okteto.OktetoContext{
		Namespace: "test",
		Registry:  "registry.okteto",
	}

	namespaceEnvVar := model.BuildArg{
		Name: model.OktetoNamespaceEnvVar, Value: context.Namespace,
	}

	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": &context,
		},
		CurrentContext: "test",
	}

	serviceContext := "service"
	serviceDockerfile := "CustomDockerfile"

	originalWd, errwd := os.Getwd()
	if errwd != nil {
		t.Fatal(errwd)
	}

	dir := t.TempDir()

	os.Chdir(dir)
	defer os.Chdir(originalWd)

	err := os.Mkdir(serviceContext, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}

	df := filepath.Join(serviceContext, serviceDockerfile)
	dockerfile, errCreate := os.Create(df)
	if errCreate != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := dockerfile.Close(); err != nil {
			t.Fatal(err)
		}
		removeFile(df)
	})

	tests := []struct {
		name        string
		serviceName string
		buildInfo   *model.BuildInfo
		isOkteto    bool
		initialOpts *types.BuildOptions
		expected    *types.BuildOptions
	}{
		{
			name:        "is-okteto-empty-buildInfo",
			serviceName: "service",
			buildInfo:   &model.BuildInfo{},
			isOkteto:    true,
			expected: &types.BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				Tag:        "okteto.dev/movies-service:okteto",
				BuildArgs:  []string{namespaceEnvVar.String()},
			},
		},
		{
			name:        "not-okteto-empty-buildInfo",
			serviceName: "service",
			buildInfo:   &model.BuildInfo{},
			isOkteto:    false,
			expected: &types.BuildOptions{
				OutputMode: oktetoLog.TTYFormat,
				BuildArgs:  []string{},
			},
		},
		{
			name:        "is-okteto-missing-image-buildInfo",
			serviceName: "service",
			buildInfo: &model.BuildInfo{
				Context:    serviceContext,
				Dockerfile: serviceDockerfile,
				Target:     "build",
				CacheFrom:  []string{"cache-image"},
				Args: model.BuildArgs{
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
			buildInfo: &model.BuildInfo{
				Context:    serviceContext,
				Dockerfile: serviceDockerfile,
				Target:     "build",
				CacheFrom:  []string{"cache-image"},
				Args: model.BuildArgs{
					{
						Name:  "arg1",
						Value: "value1",
					},
				},
				VolumesToInclude: []model.StackVolume{
					{
						LocalPath:  "a",
						RemotePath: "b",
					},
				},
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
				CacheFrom:  []string{"cache-image"},
				BuildArgs:  []string{namespaceEnvVar.String(), "arg1=value1"},
			},
		},
		{
			name:        "is-okteto-has-image-buildInfo",
			serviceName: "service",
			buildInfo: &model.BuildInfo{
				Image:      "okteto.dev/mycustomimage:dev",
				Context:    serviceContext,
				Dockerfile: serviceDockerfile,
				Target:     "build",
				CacheFrom:  []string{"cache-image"},
				Args: model.BuildArgs{
					namespaceEnvVar,
					{
						Name:  "arg1",
						Value: "value1",
					},
				},
				Secrets: map[string]string{
					"mysecret": "source",
				},
				ExportCache: "export-image",
			},
			initialOpts: &types.BuildOptions{
				OutputMode: "tty",
			},
			isOkteto: true,
			expected: &types.BuildOptions{
				OutputMode:  oktetoLog.TTYFormat,
				Tag:         "okteto.dev/mycustomimage:dev",
				File:        filepath.Join(serviceContext, serviceDockerfile),
				Target:      "build",
				Path:        "service",
				CacheFrom:   []string{"cache-image"},
				BuildArgs:   []string{namespaceEnvVar.String(), "arg1=value1"},
				Secrets:     []string{"id=mysecret,src=source"},
				ExportCache: "export-image",
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
						Registry:  "https://registry.okteto.com",
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
			result := OptsFromBuildInfo(manifest.Name, tt.serviceName, manifest.Build[tt.serviceName], tt.initialOpts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractFromContextAndDockerfile(t *testing.T) {
	buildName := "frontendTest"
	mockDir := "mockDir"

	originalWd, errwd := os.Getwd()
	if errwd != nil {
		t.Fatal(errwd)
	}

	dir := t.TempDir()

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
		optionalContext    string
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
					defer removeFile(df)
					dfFile, err := os.Create(df)
					if err != nil {
						t.Fatal(err)
					}

					log.Printf("created docker file: %s", df)

					if err := dfFile.Close(); err != nil {
						t.Fatal(err)
					}
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

			assert.Equal(t, tt.fileExpected, file)

			if tt.dockerfilesCreated != nil {
				for _, df := range tt.dockerfilesCreated {
					removeFile(df)
				}
			}

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

func Test_parseTempSecrets(t *testing.T) {
	secretDir := t.TempDir()
	tmpTestSecretFile, err := os.CreateTemp(secretDir, "")
	if err != nil {
		t.Fatal(err)
	}

	writer := bufio.NewWriter(tmpTestSecretFile)
	_, _ = writer.Write([]byte(fmt.Sprintf("%s\n", "content for ${SECRET_ENV}")))
	writer.Flush()

	tempFolder := t.TempDir()
	envValue := "secret env value"
	t.Setenv("SECRET_ENV", envValue)

	buildOpts := &types.BuildOptions{
		Secrets: []string{
			fmt.Sprintf("id=mysecret,src=%s", tmpTestSecretFile.Name()),
		},
	}

	if err := parseTempSecrets(tempFolder, buildOpts); err != nil {
		t.Fatal(err)
	}

	if err := tmpTestSecretFile.Close(); err != nil {
		t.Fatal(err)
	}

	assert.NotContains(t, buildOpts.Secrets[0], tmpTestSecretFile.Name())
	assert.Contains(t, buildOpts.Secrets[0], "secret-")

	newSecretPath := strings.SplitN(buildOpts.Secrets[0], "id=mysecret,src=", 2)
	b, err := os.ReadFile(newSecretPath[1])
	if err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, string(b), envValue)

}
