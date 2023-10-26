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
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeV1Builder struct {
	err           error
	assertOptions func(o *types.BuildOptions)
}

func (f fakeV1Builder) Build(_ context.Context, opts *types.BuildOptions) error {
	if f.assertOptions != nil {
		f.assertOptions(opts)
	}
	return f.err
}

func (f fakeV1Builder) IsV1() bool { return true }

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
		name     string
		config   config
		expected error
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
			rdc := remoteDeployCommand{
				builderV1:            fakeV1Builder{err: tt.config.builderErr},
				fs:                   fs,
				workingDirectoryCtrl: wdCtrl,
				temporalCtrl:         tempCreator,
				clusterMetadata: func(context.Context) (*types.ClusterMetadata, error) {
					return &types.ClusterMetadata{Certificate: tt.config.cert}, nil
				},
				getBuildEnvVars: func() map[string]string { return nil },
			}
			err := rdc.deploy(ctx, tt.config.options)
			if tt.expected != nil {
				assert.EqualError(t, err, tt.expected.Error())
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

	rdc := remoteDeployCommand{
		builderV1: fakeV1Builder{
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
		getBuildEnvVars: func() map[string]string { return nil },
	}

	err := rdc.deploy(ctx, &Options{
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
	rdc := remoteDeployCommand{
		sshAuthSockEnvvar:    envvarName,
		getBuildEnvVars:      func() map[string]string { return nil },
		knownHostsPath:       knowHostFile.Name(),
		builderV1:            fakeV1Builder{assertOptions: assertFn},
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/")),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
		clusterMetadata: func(context.Context) (*types.ClusterMetadata, error) {
			return &types.ClusterMetadata{}, nil
		},
	}

	err = rdc.deploy(context.Background(), &Options{
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
	rdc := remoteDeployCommand{
		sshAuthSockEnvvar:    envvarName,
		knownHostsPath:       "inexistent-file",
		builderV1:            fakeV1Builder{assertOptions: assertFn},
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/")),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
		clusterMetadata: func(context.Context) (*types.ClusterMetadata, error) {
			return &types.ClusterMetadata{}, nil
		},
		getBuildEnvVars: func() map[string]string { return nil },
	}

	err := rdc.deploy(context.Background(), &Options{
		Manifest: &model.Manifest{
			Deploy: &model.DeployInfo{
				Image: "test-image",
			},
		},
	})
	assert.NoError(t, err)
}

func TestGetDeployFlags(t *testing.T) {
	type config struct {
		opts *Options
	}
	var tests = []struct {
		name     string
		config   config
		expected []string
	}{
		{
			name: "no extra options",
			config: config{
				opts: &Options{
					Timeout: 2 * time.Minute,
				},
			},
			expected: []string{"--timeout 2m0s"},
		},
		{
			name: "name set",
			config: config{
				opts: &Options{
					Name:    "test",
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\"", "--timeout 5m0s"},
		},
		{
			name: "name multiple words",
			config: config{
				opts: &Options{
					Name:    "this is a test",
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"this is a test\"", "--timeout 5m0s"},
		},
		{
			name: "namespace set",
			config: config{
				opts: &Options{
					Namespace: "test",
					Timeout:   5 * time.Minute,
				},
			},
			expected: []string{"--namespace test", "--timeout 5m0s"},
		},
		{
			name: "manifest path set",
			config: config{
				opts: &Options{
					ManifestPathFlag: "/hello/this/is/a/test",
					Timeout:          5 * time.Minute,
				},
			},
			expected: []string{"--file /hello/this/is/a/test", "--timeout 5m0s"},
		},
		{
			name: "variables set",
			config: config{
				opts: &Options{
					Variables: []string{
						"a=b",
						"c=d",
					},
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--var a=b --var c=d", "--timeout 5m0s"},
		},
		{
			name: "wait set",
			config: config{
				opts: &Options{
					Wait:    true,
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--wait", "--timeout 5m0s"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := getDeployFlags(tt.config.opts)
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
		dockerfileName    string
		dockerfileContent string
		buildEnvVars      map[string]string
		err               error
	}
	var tests = []struct {
		name     string
		config   config
		expected expected
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
RUN mkdir -p /etc/ssl/certs/
RUN echo "$OKTETO_TLS_CERT_BASE64" | base64 -d > /etc/ssl/certs/okteto.crt

COPY . /okteto/src
WORKDIR /okteto/src


ENV OKTETO_BUIL_SVC2_IMAGE TWO_VALUE

ENV OKTETO_BUIL_SVC_IMAGE ONE_VALUE


ARG OKTETO_GIT_COMMIT
ARG OKTETO_INVALIDATE_CACHE

RUN okteto registrytoken install --force --log-output=json

RUN --mount=type=secret,id=known_hosts --mount=id=remote,type=ssh \
  mkdir -p $HOME/.ssh && echo "UserKnownHostsFile=/run/secrets/known_hosts" >> $HOME/.ssh/config && \
  okteto deploy --log-output=json --server-name="$INTERNAL_SERVER_NAME" --timeout 0s
`,
				buildEnvVars: map[string]string{"OKTETO_BUIL_SVC_IMAGE": "ONE_VALUE", "OKTETO_BUIL_SVC2_IMAGE": "TWO_VALUE"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wdCtrl.SetErrors(tt.config.wd)
			rdc := remoteDeployCommand{
				getBuildEnvVars: func() map[string]string {
					return tt.expected.buildEnvVars
				},
				fs:                   fs,
				workingDirectoryCtrl: wdCtrl,
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
	got := newRemoteDeployer(&fakeV2Builder{})
	require.IsType(t, &remoteDeployCommand{}, got)
	require.NotNil(t, got.getBuildEnvVars)
}
