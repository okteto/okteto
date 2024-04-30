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

package remote

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	filesystem "github.com/okteto/okteto/pkg/filesystem/fake"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBuilder struct {
	err           error
	assertOptions func(o *types.BuildOptions)
}

func (f fakeBuilder) Run(_ context.Context, opts *types.BuildOptions, _ *io.Controller) error {
	if f.assertOptions != nil {
		f.assertOptions(opts)
	}
	return f.err
}

func TestRemoteTest(t *testing.T) {
	ctx := context.Background()
	fakeManifest := &model.Manifest{
		Deploy: &model.DeployInfo{
			Image: "test-image",
		},
	}
	wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/"))
	fs := afero.NewMemMapFs()
	tempCreator := filesystem.NewTemporalDirectoryCtrl(fs)

	type config struct {
		wd            filesystem.FakeWorkingDirectoryCtrlErrors
		tempFsCreator error
		params        *Params
		builderErr    error
		cert          []byte
	}
	var tests = []struct {
		expected error
		name     string
		config   config
	}{
		{
			name: "OS can't access to the working directory",
			config: config{
				wd: filesystem.FakeWorkingDirectoryCtrlErrors{
					Getter: assert.AnError,
				},
				params: &Params{},
				cert:   []byte("this-is-my-cert-there-are-many-like-it-but-this-one-is-mine"),
			},
			expected: assert.AnError,
		},
		{
			name: "OS can't create temporal directory",
			config: config{
				params:        &Params{},
				tempFsCreator: assert.AnError,
			},
			expected: assert.AnError,
		},
		{
			name: "OS can't change to the previous working directory",
			config: config{
				wd: filesystem.FakeWorkingDirectoryCtrlErrors{
					Setter: assert.AnError,
				},
				params: &Params{
					Manifest: fakeManifest,
				},
			},
			expected: assert.AnError,
		},
		{
			name: "build incorrect",
			config: config{
				params: &Params{
					Manifest: fakeManifest,
				},
				builderErr: assert.AnError,
			},
			expected: oktetoErrors.UserError{
				E: assert.AnError,
			},
		},
		{
			name: "everything correct",
			config: config{
				params: &Params{
					Manifest: fakeManifest,
				},
			},
		},
	}

	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "namespace",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wdCtrl.SetErrors(tt.config.wd)
			tempCreator.SetError(tt.config.tempFsCreator)

			usersClient := client.NewFakeUsersClient(&types.User{})
			usersClient.ClusterMetadata = types.ClusterMetadata{Certificate: tt.config.cert}
			oktetoClient := &client.FakeOktetoClient{
				Users: usersClient,
			}
			rdc := Runner{
				builder:              fakeBuilder{err: tt.config.builderErr},
				fs:                   fs,
				workingDirectoryCtrl: wdCtrl,
				temporalCtrl:         tempCreator,
				oktetoClientProvider: client.NewFakeOktetoClientProvider(oktetoClient),
			}
			err := rdc.Run(ctx, tt.config.params)
			if tt.expected != nil {
				assert.ErrorContains(t, err, tt.expected.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtraHosts(t *testing.T) {
	ctx := context.Background()
	fakeManifest := &model.Manifest{
		Deploy: &model.DeployInfo{
			Image: "test-image",
		},
	}
	wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/"))
	fs := afero.NewMemMapFs()
	tempCreator := filesystem.NewTemporalDirectoryCtrl(fs)

	usersClient := client.NewFakeUsersClient(&types.User{})
	usersClient.ClusterMetadata = types.ClusterMetadata{ServerName: "1.2.3.4:443"}
	oktetoClient := &client.FakeOktetoClient{
		Users: usersClient,
	}
	rdc := Runner{
		builder: fakeBuilder{
			assertOptions: func(o *types.BuildOptions) {
				require.Len(t, o.ExtraHosts, 2)
				for _, eh := range o.ExtraHosts {
					require.Equal(t, eh.IP, "1.2.3.4")
				}
			},
		},
		fs:                   fs,
		workingDirectoryCtrl: wdCtrl,
		temporalCtrl:         tempCreator,
		oktetoClientProvider: client.NewFakeOktetoClientProvider(oktetoClient),
		useInternalNetwork:   true,
	}

	err := rdc.Run(ctx, &Params{
		Manifest: fakeManifest,
	})
	require.NoError(t, err)
}

func TestRemoteDeployWithSshAgent(t *testing.T) {
	fs := afero.NewMemMapFs()
	socket, err := os.CreateTemp("", "okteto-test-ssh-*")
	require.NoError(t, err)
	defer socket.Close()

	knowHostFile, err := os.CreateTemp("", "okteto-test-know_hosts-*")
	require.NoError(t, err)
	defer socket.Close()

	assertFn := func(o *types.BuildOptions) {
		assert.Contains(t, o.SshSessions, types.BuildSshSession{Id: "remote", Target: socket.Name()})
		assert.Contains(t, o.Secrets, fmt.Sprintf("id=known_hosts,src=%s", knowHostFile.Name()))
	}

	envvarName := fmt.Sprintf("TEST_SOCKET_%s", os.Getenv("RANDOM"))

	t.Setenv(envvarName, socket.Name())
	defer func() {
		t.Logf("cleaning up %s envvar", envvarName)
		os.Unsetenv(envvarName)
	}()

	oktetoClient := &client.FakeOktetoClient{
		Users: client.NewFakeUsersClient(&types.User{}),
	}
	rdc := Runner{
		sshAuthSockEnvvar:    envvarName,
		builder:              fakeBuilder{assertOptions: assertFn},
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/")),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
		oktetoClientProvider: client.NewFakeOktetoClientProvider(oktetoClient),
	}

	err = rdc.Run(context.Background(), &Params{
		KnownHostsPath: knowHostFile.Name(),
		Manifest: &model.Manifest{
			Deploy: &model.DeployInfo{
				Image: "test-image",
			},
		},
	})
	assert.NoError(t, err)
}

func TestRemoteDeployWithBadSshAgent(t *testing.T) {
	fs := afero.NewMemMapFs()

	assertFn := func(o *types.BuildOptions) {
		assert.NotContains(t, o.SshSessions, types.BuildSshSession{Id: "remote", Target: "bad-socket"})
		assert.NotContains(t, o.Secrets, fmt.Sprintf("id=known_hosts,src=%s", "inexistent-file"))
	}

	envvarName := fmt.Sprintf("TEST_SOCKET_%s", os.Getenv("RANDOM"))

	t.Setenv(envvarName, "bad-socket")
	defer func() {
		t.Logf("cleaning up %s envvar", envvarName)
		os.Unsetenv(envvarName)
	}()

	oktetoClient := &client.FakeOktetoClient{
		Users: client.NewFakeUsersClient(&types.User{}),
	}
	rdc := Runner{
		sshAuthSockEnvvar:    envvarName,
		builder:              fakeBuilder{assertOptions: assertFn},
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/")),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
		oktetoClientProvider: client.NewFakeOktetoClientProvider(oktetoClient),
	}

	err := rdc.Run(context.Background(), &Params{
		KnownHostsPath: "inexistent-file",
		Manifest: &model.Manifest{
			Deploy: &model.DeployInfo{
				Image: "test-image",
			},
		},
	})
	assert.NoError(t, err)
}

func TestCreateDockerfile(t *testing.T) {
	wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/"))
	fs := afero.NewMemMapFs()
	fakeManifest := &model.Manifest{
		Deploy: &model.DeployInfo{},
	}
	type config struct {
		wd     filesystem.FakeWorkingDirectoryCtrlErrors
		params *Params
	}
	type expected struct {
		err               error
		buildEnvVars      map[string]string
		dependencyEnvVars map[string]string
		dockerfileName    string
		dockerfileContent string
	}
	var tests = []struct {
		expected expected
		config   config
		name     string
	}{
		{
			name: "OS can't access working directory",
			config: config{
				wd: filesystem.FakeWorkingDirectoryCtrlErrors{
					Getter: assert.AnError,
				},
			},
			expected: expected{
				dockerfileName: "",
				err:            assert.AnError,
			},
		},
		{
			name: "with dockerignore",
			config: config{
				params: &Params{
					BaseImage:           "test-image",
					Manifest:            fakeManifest,
					BuildEnvVars:        map[string]string{"OKTETO_BUIL_SVC_IMAGE": "ONE_VALUE", "OKTETO_BUIL_SVC2_IMAGE": "TWO_VALUE"},
					DependenciesEnvVars: map[string]string{"OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD": "dependency_pass", "OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME": "dependency_user"},
					DockerfileName:      "Dockerfile.deploy",
					Command:             "deploy",
					CommandFlags:        []string{"--name \"test\""},
				},
			},
			expected: expected{
				dockerfileName: filepath.Clean("/test/Dockerfile.deploy"),
				dockerfileContent: `
FROM okteto/okteto:latest as okteto-cli

FROM test-image as runner

ENV PATH="${PATH}:/okteto/bin"
COPY --from=okteto-cli /usr/local/bin/* /okteto/bin/


ENV OKTETO_DEPLOY_REMOTE true
ARG OKTETO_NAMESPACE
ARG OKTETO_CONTEXT
ARG OKTETO_TOKEN
ARG OKTETO_ACTION_NAME
ARG OKTETO_TLS_CERT_BASE64
ARG INTERNAL_SERVER_NAME
ARG OKTETO_DEPLOYABLE
ARG GITHUB_REPOSITORY
ARG BUILDKIT_HOST
ARG OKTETO_REGISTRY_URL
ARG OKTETO_IS_PREVIEW_ENVIRONMENT
RUN mkdir -p /etc/ssl/certs/
RUN echo "$OKTETO_TLS_CERT_BASE64" | base64 -d > /etc/ssl/certs/okteto.crt

COPY . /okteto/src
WORKDIR /okteto/src


ENV OKTETO_BUIL_SVC2_IMAGE TWO_VALUE

ENV OKTETO_BUIL_SVC_IMAGE ONE_VALUE



ENV OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD dependency_pass

ENV OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME dependency_user


ARG OKTETO_GIT_COMMIT
ARG OKTETO_GIT_BRANCH
ARG OKTETO_INVALIDATE_CACHE

RUN okteto registrytoken install --force --log-output=json

RUN \
  \
  --mount=type=secret,id=known_hosts --mount=id=remote,type=ssh \
  mkdir -p $HOME/.ssh && echo "UserKnownHostsFile=/run/secrets/known_hosts" >> $HOME/.ssh/config && \
  /okteto/bin/okteto remote-run deploy --log-output=json --server-name="$INTERNAL_SERVER_NAME" --name "test"
`,
				buildEnvVars:      map[string]string{"OKTETO_BUIL_SVC_IMAGE": "ONE_VALUE", "OKTETO_BUIL_SVC2_IMAGE": "TWO_VALUE"},
				dependencyEnvVars: map[string]string{"OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD": "dependency_pass", "OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME": "dependency_user"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wdCtrl.SetErrors(tt.config.wd)
			rdc := Runner{
				fs:                   fs,
				workingDirectoryCtrl: wdCtrl,
			}
			dockerfileName, err := rdc.createDockerfile("/test", tt.config.params)
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.dockerfileName, dockerfileName)

			if dockerfileName != "" {
				bFile, err := afero.ReadFile(fs, dockerfileName)
				assert.NoError(t, err)
				assert.EqualValues(t, tt.expected.dockerfileContent, string(bFile))
			}

			if tt.expected.err == nil {
				_, err = rdc.fs.Stat(filepath.Join("/test", tt.config.params.DockerfileName))
				assert.NoError(t, err)
			}

		})
	}
}

func TestDockerfileWithCache(t *testing.T) {
	wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/"))
	fs := afero.NewMemMapFs()
	rdc := Runner{
		fs:                   fs,
		workingDirectoryCtrl: wdCtrl,
	}
	caches := []string{"/my", "/cache", "/list"}
	dockerfileName, err := rdc.createDockerfile("/test", &Params{
		Caches:         caches,
		DockerfileName: "myDockerfile",
	})
	require.NoError(t, err)
	require.Equal(t, filepath.Clean("/test/myDockerfile"), dockerfileName)
	d, err := afero.ReadFile(fs, dockerfileName)
	require.NoError(t, err)
	for _, cache := range caches {
		pattern := fmt.Sprintf("--mount=type=cache,target=%s", cache)
		ok, err := regexp.MatchString(pattern, string(d))
		require.NoError(t, err)
		require.True(t, ok)
	}
}

func Test_getOktetoCLIVersion(t *testing.T) {
	var tests = []struct {
		name                                 string
		versionString, expected, cliImageEnv string
	}{
		{
			name:          "no version string and no env return latest",
			versionString: "",
			expected:      "okteto/okteto:latest",
		},
		{
			name:          "no version string return env value",
			versionString: "",
			cliImageEnv:   "okteto/remote:test",
			expected:      "okteto/remote:test",
		},
		{
			name:          "found version string",
			versionString: "2.2.2",
			expected:      "okteto/okteto:2.2.2",
		},
		{
			name:          "found incorrect version string return latest ",
			versionString: "2.a.2",
			expected:      "okteto/okteto:latest",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.cliImageEnv != "" {
				t.Setenv(constants.OktetoDeployRemoteImage, tt.cliImageEnv)
			}

			version := getOktetoCLIVersion(tt.versionString)
			require.Equal(t, version, tt.expected)
		})
	}
}

func TestGetExtraHosts(t *testing.T) {
	registryURL := "registry.test.dev.okteto.net"
	subdomain := "test.dev.okteto.net"
	ip := "1.2.3.4"

	var tests = []struct {
		name     string
		expected []types.HostMap
		metadata types.ClusterMetadata
	}{
		{
			name:     "no metadata information",
			metadata: types.ClusterMetadata{},
			expected: []types.HostMap{
				{Hostname: registryURL, IP: ip},
				{Hostname: fmt.Sprintf("kubernetes.%s", subdomain), IP: ip},
			},
		},
		{
			name: "with buildkit internal ip",
			metadata: types.ClusterMetadata{
				BuildKitInternalIP: "4.3.2.1",
			},
			expected: []types.HostMap{
				{Hostname: registryURL, IP: ip},
				{Hostname: fmt.Sprintf("kubernetes.%s", subdomain), IP: ip},
				{Hostname: fmt.Sprintf("buildkit.%s", subdomain), IP: "4.3.2.1"},
			},
		},
		{
			name: "with public domain",
			metadata: types.ClusterMetadata{
				PublicDomain: "publicdomain.dev.okteto.net",
			},
			expected: []types.HostMap{
				{Hostname: registryURL, IP: ip},
				{Hostname: fmt.Sprintf("kubernetes.%s", subdomain), IP: ip},
				{Hostname: "publicdomain.dev.okteto.net", IP: ip},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extraHosts := getExtraHosts(registryURL, subdomain, ip, tt.metadata)

			assert.EqualValues(t, tt.expected, extraHosts)
		})
	}
}

func TestGetContextPath(t *testing.T) {
	cwd := filepath.Clean("/path/to/current/directory")

	rd := Runner{
		fs: afero.NewMemMapFs(),
	}

	t.Run("Manifest path is empty", func(t *testing.T) {
		expected := cwd
		result := rd.getContextPath(cwd, "")
		assert.Equal(t, expected, result)
	})

	t.Run("Manifest path is a absolute path and directory", func(t *testing.T) {
		manifestPath := filepath.Clean("/path/to/current/directory")
		expected := manifestPath
		rd.fs = afero.NewMemMapFs()
		rd.fs.MkdirAll(manifestPath, 0755)
		result := rd.getContextPath(cwd, manifestPath)
		assert.Equal(t, expected, result)
	})

	t.Run("Manifest path is a file and absolute path", func(t *testing.T) {
		manifestPath := filepath.Clean("/path/to/current/directory/file.yaml")
		expected := filepath.Clean("/path/to/current/directory")
		rd.fs = afero.NewMemMapFs()
		rd.fs.MkdirAll(expected, 0755)
		rd.fs.Create(manifestPath)
		result := rd.getContextPath(cwd, manifestPath)
		assert.Equal(t, expected, result)
	})

	t.Run("Manifest path is pointing to a file in the .okteto folder and absolute path", func(t *testing.T) {
		manifestPath := filepath.Clean("/path/to/current/directory/.okteto/file.yaml")
		expected := filepath.Clean("/path/to/current/directory")
		rd.fs = afero.NewMemMapFs()
		rd.fs.MkdirAll(expected, 0755)
		rd.fs.Create(manifestPath)
		result := rd.getContextPath(cwd, manifestPath)
		assert.Equal(t, expected, result)
	})

	t.Run("Manifest path does not exist", func(t *testing.T) {
		expected := cwd
		result := rd.getContextPath(cwd, "nonexistent.yaml")
		assert.Equal(t, expected, result)
	})
}

func TestGetOriginalCWD(t *testing.T) {

	t.Run("error getting the working directory", func(t *testing.T) {
		wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/"))
		wdCtrl.SetErrors(filesystem.FakeWorkingDirectoryCtrlErrors{
			Getter: assert.AnError,
		})
		r := &Runner{
			workingDirectoryCtrl: wdCtrl,
		}

		_, err := r.getOriginalCWD("")

		require.Error(t, err)
	})

	t.Run("with empty manifest path", func(t *testing.T) {
		wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/tmp/test"))
		r := &Runner{
			workingDirectoryCtrl: wdCtrl,
		}

		result, err := r.getOriginalCWD("")
		expected := filepath.Clean("/tmp/test")

		require.NoError(t, err)
		require.Equal(t, expected, result)
	})

	t.Run("with manifest path to a dir", func(t *testing.T) {
		wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/tmp/test"))
		r := &Runner{
			workingDirectoryCtrl: wdCtrl,
		}

		path := filepath.Join("test", ".okteto")
		result, err := r.getOriginalCWD(path)

		expected := filepath.Clean("/tmp")
		require.NoError(t, err)
		require.Equal(t, expected, result)
	})

	t.Run("with manifest path to a file", func(t *testing.T) {
		wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/tmp/test"))
		r := &Runner{
			workingDirectoryCtrl: wdCtrl,
		}

		path := filepath.Join("test", "okteto.yml")
		result, err := r.getOriginalCWD(path)

		expected := filepath.Clean("/tmp")
		require.NoError(t, err)
		require.Equal(t, expected, result)
	})
}

func TestCreateDockerignoreFileWithFilesystem(t *testing.T) {
	dockerignoreWd := "/test/"

	type config struct {
		wd string
	}
	var tests = []struct {
		name            string
		expectedContent string
		config          config
	}{
		{
			name: "dockerignore present copy .oktetodeployignore to .dockerignore without manifest",
			config: config{
				wd: dockerignoreWd,
			},
			expectedContent: "FROM alpine",
		},
		{
			name: "dockerignore present copy .oktetodeployignore to .dockerignore",
			config: config{
				wd: dockerignoreWd,
			},
			expectedContent: "FROM alpine",
		},
		{
			name:            "without dockerignore",
			config:          config{},
			expectedContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			fs := afero.NewMemMapFs()

			assert.NoError(t, fs.MkdirAll(dockerignoreWd, 0755))
			assert.NoError(t, afero.WriteFile(fs, filepath.Join(dockerignoreWd, ".oktetodeployignore"), []byte("FROM alpine"), 0644))

			err := CreateDockerignoreFileWithFilesystem(tt.config.wd, tempDir, fs)
			assert.NoError(t, err)
			b, err := afero.ReadFile(fs, filepath.Join(tempDir, ".dockerignore"))
			assert.Equal(t, tt.expectedContent, string(b))
			assert.NoError(t, err)

		})
	}
}
