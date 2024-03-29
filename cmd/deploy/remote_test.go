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

package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	filesystem "github.com/okteto/okteto/pkg/filesystem/fake"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRemoteRunner struct {
	err           error
	assertOptions func(o *types.BuildOptions)
}

func (f fakeRemoteRunner) Run(_ context.Context, opts *types.BuildOptions, _ *io.Controller) error {
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
		options       *Options
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
				options: &Options{},
				cert:    []byte("this-is-my-cert-there-are-many-like-it-but-this-one-is-mine"),
			},
			expected: assert.AnError,
		},
		{
			name: "OS can't create temporal directory",
			config: config{
				options:       &Options{},
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
				options: &Options{
					Manifest: fakeManifest,
				},
			},
			expected: assert.AnError,
		},
		{
			name: "build incorrect",
			config: config{
				options: &Options{
					Manifest: fakeManifest,
				},
				builderErr: assert.AnError,
			},
			expected: oktetoErrors.UserError{
				E: assert.AnError,
			},
		},
		{
			name: "build with command error",
			config: config{
				options: &Options{
					Manifest: fakeManifest,
				},
				builderErr: build.OktetoCommandErr{
					Stage: "test",
					Err:   assert.AnError,
				},
			},
			expected: oktetoErrors.UserError{
				E: assert.AnError,
			},
		},
		{
			name: "everything correct",
			config: config{
				options: &Options{
					Manifest: fakeManifest,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wdCtrl.SetErrors(tt.config.wd)
			tempCreator.SetError(tt.config.tempFsCreator)
			rdc := remoteDeployer{
				runner:               fakeRemoteRunner{err: tt.config.builderErr},
				fs:                   fs,
				workingDirectoryCtrl: wdCtrl,
				temporalCtrl:         tempCreator,
				clusterMetadata: func(context.Context) (*types.ClusterMetadata, error) {
					return &types.ClusterMetadata{Certificate: tt.config.cert}, nil
				},
				getBuildEnvVars:      func() map[string]string { return nil },
				getDependencyEnvVars: func(_ environGetter) map[string]string { return nil },
			}
			err := rdc.Deploy(ctx, tt.config.options)
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

	rdc := remoteDeployer{
		runner: fakeRemoteRunner{
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
		clusterMetadata: func(context.Context) (*types.ClusterMetadata, error) {
			return &types.ClusterMetadata{
				ServerName: "1.2.3.4:443",
			}, nil
		},
		getBuildEnvVars:      func() map[string]string { return nil },
		getDependencyEnvVars: func(_ environGetter) map[string]string { return nil },
		useInternalNetwork:   true,
	}

	err := rdc.Deploy(ctx, &Options{
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
	rdc := remoteDeployer{
		sshAuthSockEnvvar:    envvarName,
		getBuildEnvVars:      func() map[string]string { return nil },
		knownHostsPath:       knowHostFile.Name(),
		runner:               fakeRemoteRunner{assertOptions: assertFn},
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/")),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
		clusterMetadata: func(context.Context) (*types.ClusterMetadata, error) {
			return &types.ClusterMetadata{}, nil
		},
		getDependencyEnvVars: func(_ environGetter) map[string]string { return nil },
	}

	err = rdc.Deploy(context.Background(), &Options{
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
	rdc := remoteDeployer{
		sshAuthSockEnvvar:    envvarName,
		knownHostsPath:       "inexistent-file",
		runner:               fakeRemoteRunner{assertOptions: assertFn},
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/")),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
		clusterMetadata: func(context.Context) (*types.ClusterMetadata, error) {
			return &types.ClusterMetadata{}, nil
		},
		getBuildEnvVars:      func() map[string]string { return nil },
		getDependencyEnvVars: func(_ environGetter) map[string]string { return nil },
	}

	err := rdc.Deploy(context.Background(), &Options{
		Manifest: &model.Manifest{
			Deploy: &model.DeployInfo{
				Image: "test-image",
			},
		},
	})
	assert.NoError(t, err)
}

func TestGetCommandFlags(t *testing.T) {
	type config struct {
		opts *Options
	}
	var tests = []struct {
		name      string
		config    config
		expected  []string
		expectErr bool
	}{
		{
			name: "no extra options",
			config: config{
				opts: &Options{
					Timeout: 2 * time.Minute,
					Name:    "test",
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "name set",
			config: config{
				opts: &Options{
					Name:    "test",
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "name multiple words",
			config: config{
				opts: &Options{
					Name:    "this is a test",
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"this is a test\""},
		},
		{
			name: "namespace is not set",
			config: config{
				opts: &Options{
					Name:      "test",
					Namespace: "test",
					Timeout:   5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "manifest path is not set",
			config: config{
				opts: &Options{
					Name:             "test",
					ManifestPathFlag: "/hello/this/is/a/test",
					ManifestPath:     "/hello/this/is/a/test",
					Timeout:          5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "variables set",
			config: config{
				opts: &Options{
					Name: "test",
					Variables: []string{
						"a=b",
						"c=d",
					},
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\"", "--var a=\"b\" --var c=\"d\""},
		},
		{
			name: "wait is not set",
			config: config{
				opts: &Options{
					Name:    "test",
					Wait:    true,
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "multiword var value",
			config: config{
				opts: &Options{
					Name:      "test",
					Variables: []string{"test=multi word value"},
				},
			},
			expected: []string{"--name \"test\"", "--var test=\"multi word value\""},
		},
		{
			name: "wrong multiword var value",
			config: config{
				opts: &Options{
					Name:      "test",
					Variables: []string{"test -> multi word value"},
				},
			},
			expected:  nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := getCommandFlags(tt.config.opts)
			if tt.expectErr {
				require.Error(t, err)
			}
			assert.Equal(t, tt.expected, flags)
		})
	}
}

func TestCreateDockerfile(t *testing.T) {
	wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/"))
	fs := afero.NewMemMapFs()
	fakeManifest := &model.Manifest{
		Deploy: &model.DeployInfo{
			Image: "test-image",
		},
	}
	type config struct {
		wd   filesystem.FakeWorkingDirectoryCtrlErrors
		opts *Options
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
				opts: &Options{
					Name:     "test",
					Manifest: fakeManifest,
				},
			},
			expected: expected{
				dockerfileName: filepath.Clean("/test/Dockerfile.deploy"),
				dockerfileContent: `
FROM okteto/okteto:latest as okteto-cli

FROM test-image as deploy

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

RUN --mount=type=secret,id=known_hosts --mount=id=remote,type=ssh \
  mkdir -p $HOME/.ssh && echo "UserKnownHostsFile=/run/secrets/known_hosts" >> $HOME/.ssh/config && \
  /okteto/bin/okteto remote-run --log-output=json --server-name="$INTERNAL_SERVER_NAME" --name "test"
`,
				buildEnvVars:      map[string]string{"OKTETO_BUIL_SVC_IMAGE": "ONE_VALUE", "OKTETO_BUIL_SVC2_IMAGE": "TWO_VALUE"},
				dependencyEnvVars: map[string]string{"OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD": "dependency_pass", "OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME": "dependency_user"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wdCtrl.SetErrors(tt.config.wd)
			rdc := remoteDeployer{
				getBuildEnvVars: func() map[string]string {
					return tt.expected.buildEnvVars
				},
				fs:                   fs,
				workingDirectoryCtrl: wdCtrl,
				getDependencyEnvVars: func(_ environGetter) map[string]string {
					return tt.expected.dependencyEnvVars
				},
			}
			dockerfileName, err := rdc.createDockerfile("/test", tt.config.opts)
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.dockerfileName, dockerfileName)

			if dockerfileName != "" {
				bFile, err := afero.ReadFile(fs, dockerfileName)
				assert.NoError(t, err)
				assert.EqualValues(t, tt.expected.dockerfileContent, string(bFile))
			}

			if tt.expected.err == nil {
				_, err = rdc.fs.Stat(filepath.Join("/test", dockerfileTemporalName))
				assert.NoError(t, err)
			}

		})
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

func Test_newRemoteDeployer(t *testing.T) {
	getBuildEnvVars := func() map[string]string { return nil }
	getDependencyEnvVars := func(_ environGetter) map[string]string { return nil }
	got := newRemoteDeployer(getBuildEnvVars, io.NewIOController(), getDependencyEnvVars)
	require.IsType(t, &remoteDeployer{}, got)
	require.NotNil(t, got.getBuildEnvVars)
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

	rd := remoteDeployer{
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
		deployCommand := &remoteDeployer{
			workingDirectoryCtrl: wdCtrl,
		}

		_, err := deployCommand.getOriginalCWD("")

		require.Error(t, err)
	})

	t.Run("with empty manifest path", func(t *testing.T) {
		wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/tmp/test"))
		deployCommand := &remoteDeployer{
			workingDirectoryCtrl: wdCtrl,
		}

		result, err := deployCommand.getOriginalCWD("")
		expected := filepath.Clean("/tmp/test")

		require.NoError(t, err)
		require.Equal(t, expected, result)
	})

	t.Run("with manifest path to a dir", func(t *testing.T) {
		wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/tmp/test"))
		deployCommand := &remoteDeployer{
			workingDirectoryCtrl: wdCtrl,
		}

		path := filepath.Join("test", ".okteto")
		result, err := deployCommand.getOriginalCWD(path)

		expected := filepath.Clean("/tmp")
		require.NoError(t, err)
		require.Equal(t, expected, result)
	})

	t.Run("with manifest path to a file", func(t *testing.T) {
		wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/tmp/test"))
		deployCommand := &remoteDeployer{
			workingDirectoryCtrl: wdCtrl,
		}

		path := filepath.Join("test", "okteto.yml")
		result, err := deployCommand.getOriginalCWD(path)

		expected := filepath.Clean("/tmp")
		require.NoError(t, err)
		require.Equal(t, expected, result)
	})
}
