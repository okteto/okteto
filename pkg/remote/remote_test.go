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

type fakeNameGenerator struct {
	name string
}

func (f fakeNameGenerator) GenerateName() string { return f.name }

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
			nameGenerator := fakeNameGenerator{
				name: "test",
			}
			rdc := Runner{
				builder:              fakeBuilder{err: tt.config.builderErr},
				fs:                   fs,
				workingDirectoryCtrl: wdCtrl,
				temporalCtrl:         tempCreator,
				oktetoClientProvider: client.NewFakeOktetoClientProvider(oktetoClient),
				ioCtrl:               io.NewIOController(),
				getEnviron: func() []string {
					return []string{}
				},
				generateSocketName: nameGenerator.GenerateName,
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
	nameGenerator := fakeNameGenerator{
		name: "test",
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
		ioCtrl:               io.NewIOController(),
		getEnviron: func() []string {
			return []string{}
		},
		generateSocketName: nameGenerator.GenerateName,
	}

	err := rdc.Run(ctx, &Params{
		Manifest: fakeManifest,
	})
	require.NoError(t, err)
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
					BuildEnvVars:        map[string]string{"OKTETO_BUIL_SVC_IMAGE": "ONE_VALUE", "OKTETO_BUILD_SVC2_IMAGE": "TWO_VALUE"},
					DependenciesEnvVars: map[string]string{"OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD": "dependency_pass", "OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME": "dependency_user"},
					OktetoCommandSpecificEnvVars: map[string]string{
						constants.OktetoIsPreviewEnvVar: "true",
					},
					DockerfileName:   "Dockerfile.deploy",
					Command:          "deploy",
					SSHAgentHostname: "ssh-agent.default.svc.cluster.local",
					SSHAgentPort:     "3000",
					CommandFlags:     []string{"--name \"test\""},
				},
			},
			expected: expected{
				dockerfileName: filepath.Clean("/test/Dockerfile.deploy"),
				dockerfileContent: `
FROM okteto/okteto:stable as okteto-cli

FROM test-image as runner

USER 0
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


ENV OKTETO_BUILD_SVC2_IMAGE="TWO_VALUE"

ENV OKTETO_BUIL_SVC_IMAGE="ONE_VALUE"


ENV OKTETO_IS_PREVIEW_ENVIRONMENT=true


ENV OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD="dependency_pass"

ENV OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME="dependency_user"


ENV OKTETO_SSH_AGENT_HOSTNAME="ssh-agent.default.svc.cluster.local"
ENV OKTETO_SSH_AGENT_PORT="3000"

ARG OKTETO_GIT_COMMIT
ARG OKTETO_GIT_BRANCH
ARG OKTETO_INVALIDATE_CACHE

RUN echo "$OKTETO_INVALIDATE_CACHE" > /etc/.oktetocachekey
RUN okteto registrytoken install --force --log-output=json

RUN \
  \
  --mount=type=secret,id=known_hosts \
  --mount=type=secret,id=OKTETO_SSH_AGENT_SOCKET,env=SSH_AUTH_SOCK \
  mkdir -p $HOME/.ssh && echo "UserKnownHostsFile=/run/secrets/known_hosts $HOME/.ssh/known_hosts" >> $HOME/.ssh/config && \
  /okteto/bin/okteto remote-run deploy --log-output=json --server-name="$INTERNAL_SERVER_NAME" --name "test"



FROM scratch
COPY --from=runner /etc/.oktetocachekey .oktetocachekey

`,
				buildEnvVars:      map[string]string{"OKTETO_BUIL_SVC_IMAGE": "ONE_VALUE", "OKTETO_BUILD_SVC2_IMAGE": "TWO_VALUE"},
				dependencyEnvVars: map[string]string{"OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD": "dependency_pass", "OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME": "dependency_user"},
			},
		},
		{
			name: "okteto test",
			config: config{
				params: &Params{
					BaseImage:           "test-image",
					Manifest:            fakeManifest,
					BuildEnvVars:        map[string]string{"OKTETO_BUIL_SVC_IMAGE": "ONE_VALUE", "OKTETO_BUILD_SVC2_IMAGE": "TWO_VALUE"},
					DependenciesEnvVars: map[string]string{"OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD": "dependency_pass", "OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME": "dependency_user"},
					OktetoCommandSpecificEnvVars: map[string]string{
						constants.CIEnvVar: "true",
					},
					DockerfileName:   "Dockerfile.test",
					Command:          "test",
					SSHAgentHostname: "ssh-agent.default.svc.cluster.local",
					SSHAgentPort:     "3000",
					CommandFlags:     []string{"--name \"test\""},
					Artifacts: []model.Artifact{
						{
							Path:        "coverage.txt",
							Destination: "coverage.txt",
						},
						{
							Path:        "report.json",
							Destination: "/testing/report.json",
						},
					},
				},
			},
			expected: expected{
				dockerfileName: filepath.Clean("/test/Dockerfile.test"),
				dockerfileContent: `
FROM okteto/okteto:stable as okteto-cli

FROM test-image as runner

USER 0
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


ENV OKTETO_BUILD_SVC2_IMAGE="TWO_VALUE"

ENV OKTETO_BUIL_SVC_IMAGE="ONE_VALUE"


ENV CI=true


ENV OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD="dependency_pass"

ENV OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME="dependency_user"


ENV OKTETO_SSH_AGENT_HOSTNAME="ssh-agent.default.svc.cluster.local"
ENV OKTETO_SSH_AGENT_PORT="3000"

ARG OKTETO_GIT_COMMIT
ARG OKTETO_GIT_BRANCH
ARG OKTETO_INVALIDATE_CACHE

RUN echo "$OKTETO_INVALIDATE_CACHE" > /etc/.oktetocachekey
RUN okteto registrytoken install --force --log-output=json

RUN \
  \
  --mount=type=secret,id=known_hosts \
  --mount=type=secret,id=OKTETO_SSH_AGENT_SOCKET,env=SSH_AUTH_SOCK \
  mkdir -p $HOME/.ssh && echo "UserKnownHostsFile=/run/secrets/known_hosts $HOME/.ssh/known_hosts" >> $HOME/.ssh/config && \
  /okteto/bin/okteto remote-run test --log-output=json --server-name="$INTERNAL_SERVER_NAME" --name "test" || true

RUN mkdir -p /okteto/artifacts

RUN if [ -e /okteto/src/coverage.txt ]; then \
    mkdir -p $(dirname /okteto/artifacts/coverage.txt) && \
    cp -r /okteto/src/coverage.txt /okteto/artifacts/coverage.txt; \
  fi

RUN if [ -e /okteto/src/report.json ]; then \
    mkdir -p $(dirname /okteto/artifacts//testing/report.json) && \
    cp -r /okteto/src/report.json /okteto/artifacts//testing/report.json; \
  fi


FROM scratch
COPY --from=runner /okteto/artifacts/ /

`,
				buildEnvVars:      map[string]string{"OKTETO_BUIL_SVC_IMAGE": "ONE_VALUE", "OKTETO_BUILD_SVC2_IMAGE": "TWO_VALUE"},
				dependencyEnvVars: map[string]string{"OKTETO_DEPENDENCY_DATABASE_VARIABLE_PASSWORD": "dependency_pass", "OKTETO_DEPENDENCY_DATABASE_VARIABLE_USERNAME": "dependency_user"},
			},
		},
		{
			name: "without ssh-agent hostname",
			config: config{
				params: &Params{
					BaseImage:                    "test-image",
					Manifest:                     fakeManifest,
					BuildEnvVars:                 map[string]string{},
					DependenciesEnvVars:          map[string]string{},
					OktetoCommandSpecificEnvVars: map[string]string{},
					DockerfileName:               "Dockerfile.deploy",
					Command:                      "deploy",
					CommandFlags:                 []string{"--name \"test\""},
				},
			},
			expected: expected{
				dockerfileName: filepath.Clean("/test/Dockerfile.deploy"),
				dockerfileContent: `
FROM okteto/okteto:stable as okteto-cli

FROM test-image as runner

USER 0
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





ENV OKTETO_SSH_AGENT_HOSTNAME=""
ENV OKTETO_SSH_AGENT_PORT=""

ARG OKTETO_GIT_COMMIT
ARG OKTETO_GIT_BRANCH
ARG OKTETO_INVALIDATE_CACHE

RUN echo "$OKTETO_INVALIDATE_CACHE" > /etc/.oktetocachekey
RUN okteto registrytoken install --force --log-output=json

RUN \
  \
  --mount=type=secret,id=known_hosts \
  --mount=type=secret,id=OKTETO_SSH_AGENT_SOCKET,env=SSH_AUTH_SOCK \
  mkdir -p $HOME/.ssh && echo "UserKnownHostsFile=/run/secrets/known_hosts $HOME/.ssh/known_hosts" >> $HOME/.ssh/config && \
  /okteto/bin/okteto remote-run deploy --log-output=json --server-name="$INTERNAL_SERVER_NAME" --name "test"



FROM scratch
COPY --from=runner /etc/.oktetocachekey .oktetocachekey

`,
				buildEnvVars:      map[string]string{},
				dependencyEnvVars: map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wdCtrl.SetErrors(tt.config.wd)
			nameGenerator := fakeNameGenerator{
				name: "okteto-socket.sock",
			}
			rdc := Runner{
				fs:                   fs,
				workingDirectoryCtrl: wdCtrl,
				getEnviron: func() []string {
					return []string{}
				},
				ioCtrl:             io.NewIOController(),
				generateSocketName: nameGenerator.GenerateName,
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
	nameGenerator := fakeNameGenerator{
		name: "okteto-socket.sock",
	}
	fs := afero.NewMemMapFs()
	rdc := Runner{
		fs:                   fs,
		workingDirectoryCtrl: wdCtrl,
		getEnviron: func() []string {
			return []string{}
		},
		ioCtrl:             io.NewIOController(),
		generateSocketName: nameGenerator.GenerateName,
	}
	caches := []string{"/my", "/cache", "/list"}
	manifest := &model.Manifest{Name: "test-manifest"}
	dockerfileName, err := rdc.createDockerfile("/test", &Params{
		Caches:         caches,
		DockerfileName: "myDockerfile",
		TestName:       "unit-test",
		Manifest:       manifest,
	})
	require.NoError(t, err)
	require.Equal(t, filepath.Clean("/test/myDockerfile"), dockerfileName)
	d, err := afero.ReadFile(fs, dockerfileName)
	require.NoError(t, err)

	// Verify that each cache has a unique ID based on manifest name, test name, and cache index
	for i, cache := range caches {
		expectedID := fmt.Sprintf("-test-manifest-unit-test-%d", i)
		pattern := fmt.Sprintf("--mount=type=cache,id=%s,target=%s", expectedID, cache)
		ok, err := regexp.MatchString(pattern, string(d))
		require.NoError(t, err)
		require.True(t, ok, "Expected cache mount pattern not found: %s", pattern)
	}
}

func TestCacheIsolationBetweenTestContexts(t *testing.T) {
	wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/"))
	nameGenerator := fakeNameGenerator{
		name: "okteto-socket.sock",
	}
	fs := afero.NewMemMapFs()
	rdc := Runner{
		fs:                   fs,
		workingDirectoryCtrl: wdCtrl,
		getEnviron: func() []string {
			return []string{}
		},
		ioCtrl:             io.NewIOController(),
		generateSocketName: nameGenerator.GenerateName,
	}
	caches := []string{"/root/.cache", "/go/pkg/mod"}

	// Test case 1: manifest1, test1
	manifest1 := &model.Manifest{Name: "app1"}
	dockerfile1, err := rdc.createDockerfile("/test1", &Params{
		Caches:         caches,
		DockerfileName: "Dockerfile.test1",
		TestName:       "unit",
		Manifest:       manifest1,
	})
	require.NoError(t, err)

	// Test case 2: manifest1, test2 (different test context)
	dockerfile2, err := rdc.createDockerfile("/test2", &Params{
		Caches:         caches,
		DockerfileName: "Dockerfile.test2",
		TestName:       "integration",
		Manifest:       manifest1,
	})
	require.NoError(t, err)

	// Test case 3: manifest2, test1 (different manifest)
	manifest2 := &model.Manifest{Name: "app2"}
	dockerfile3, err := rdc.createDockerfile("/test3", &Params{
		Caches:         caches,
		DockerfileName: "Dockerfile.test3",
		TestName:       "unit",
		Manifest:       manifest2,
	})
	require.NoError(t, err)

	// Read all dockerfiles
	d1, err := afero.ReadFile(fs, dockerfile1)
	require.NoError(t, err)
	d2, err := afero.ReadFile(fs, dockerfile2)
	require.NoError(t, err)
	d3, err := afero.ReadFile(fs, dockerfile3)
	require.NoError(t, err)

	// Verify that each test context generates unique cache IDs
	for i, cache := range caches {
		// Expected cache IDs for each context
		expectedID1 := fmt.Sprintf("-app1-unit-%d", i)
		expectedID2 := fmt.Sprintf("-app1-integration-%d", i)
		expectedID3 := fmt.Sprintf("-app2-unit-%d", i)

		// Verify dockerfile1 has app1-unit cache IDs
		pattern1 := fmt.Sprintf("--mount=type=cache,id=%s,target=%s", expectedID1, cache)
		ok, err := regexp.MatchString(pattern1, string(d1))
		require.NoError(t, err)
		require.True(t, ok, "Dockerfile1 should have cache ID %s", expectedID1)

		// Verify dockerfile2 has app1-integration cache IDs
		pattern2 := fmt.Sprintf("--mount=type=cache,id=%s,target=%s", expectedID2, cache)
		ok, err = regexp.MatchString(pattern2, string(d2))
		require.NoError(t, err)
		require.True(t, ok, "Dockerfile2 should have cache ID %s", expectedID2)

		// Verify dockerfile3 has app2-unit cache IDs
		pattern3 := fmt.Sprintf("--mount=type=cache,id=%s,target=%s", expectedID3, cache)
		ok, err = regexp.MatchString(pattern3, string(d3))
		require.NoError(t, err)
		require.True(t, ok, "Dockerfile3 should have cache ID %s", expectedID3)

		// Ensure that the cache IDs are different between contexts
		require.NotEqual(t, expectedID1, expectedID2, "Cache IDs should be different between test contexts")
		require.NotEqual(t, expectedID1, expectedID3, "Cache IDs should be different between manifests")
		require.NotEqual(t, expectedID2, expectedID3, "Cache IDs should be different between different manifest-test combinations")
	}
}

func TestGetExtraHosts(t *testing.T) {
	registryURL := "registry.test.dev.okteto.net"
	subdomain := "test.dev.okteto.net"
	ip := "1.2.3.4"

	var tests = []struct {
		name         string
		expected     []types.HostMap
		definedHosts []model.Host
		metadata     types.ClusterMetadata
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
		{
			name: "with defined hosts",
			metadata: types.ClusterMetadata{
				PublicDomain: "publicdomain.dev.okteto.net",
			},
			definedHosts: []model.Host{
				{
					Hostname: "test.dev.okteto.net",
					IP:       ip,
				},
				{
					Hostname: "test2.dev.okteto.net",
					IP:       ip,
				},
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
		_, err := GetOriginalCWD(wdCtrl, "")

		require.Error(t, err)
	})

	t.Run("with empty manifest path", func(t *testing.T) {
		wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/tmp/test"))
		result, err := GetOriginalCWD(wdCtrl, "")
		expected := filepath.Clean("/tmp/test")

		require.NoError(t, err)
		require.Equal(t, expected, result)
	})

	t.Run("with manifest path to a dir", func(t *testing.T) {
		wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/tmp/test"))
		path := filepath.Join("test", ".okteto")
		result, err := GetOriginalCWD(wdCtrl, path)

		expected := filepath.Clean("/tmp")
		require.NoError(t, err)
		require.Equal(t, expected, result)
	})

	t.Run("with manifest path to a file", func(t *testing.T) {
		wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/tmp/test"))

		path := filepath.Join("test", "okteto.yml")
		result, err := GetOriginalCWD(wdCtrl, path)

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
		rules           []string
		useDeployIgnore bool
	}{
		{
			name: "dockerignore present copy .oktetodeployignore to .dockerignore without manifest",
			config: config{
				wd: dockerignoreWd,
			},
			expectedContent: "test/*\n",
			useDeployIgnore: true,
		},
		{
			name: "dockerignore present copy .oktetodeployignore to .dockerignore with rules",
			config: config{
				wd: dockerignoreWd,
			},
			expectedContent: "test/*\nbackend?\nfrontend/**\n",
			rules:           []string{"backend?", "frontend/**"},
			useDeployIgnore: true,
		},
		{
			name:            "without dockerignore",
			config:          config{},
			expectedContent: "# Okteto docker ignore\n",
			useDeployIgnore: true,
		},
		{
			name: "dockerignore present copy .oktetodeployignore to .dockerignore with rules",
			config: config{
				wd: dockerignoreWd,
			},
			expectedContent: "backend?\nfrontend/**\n",
			rules:           []string{"backend?", "frontend/**"},
			useDeployIgnore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			fs := afero.NewMemMapFs()

			assert.NoError(t, fs.MkdirAll(dockerignoreWd, 0755))
			assert.NoError(t, afero.WriteFile(fs, filepath.Join(dockerignoreWd, ".oktetodeployignore"), []byte("test/*"), 0644))

			err := createDockerignoreFileWithFilesystem(tt.config.wd, tempDir, tt.rules, tt.useDeployIgnore, fs)
			assert.NoError(t, err)
			b, err := afero.ReadFile(fs, filepath.Join(tempDir, ".dockerignore"))
			assert.Equal(t, tt.expectedContent, string(b))
			assert.NoError(t, err)

		})
	}
}

func TestGetOktetoPrefixEnvVars(t *testing.T) {
	expectedEnvVars := map[string]string{
		"OKTETO_ENV_VAR_1":                        "value1",
		"OKTETO_ENV_VAR_2":                        "value2",
		"OKTETO_ENV_VAR_WITHOUT_VALUE_IS_SKIPPED": "MYVALUEWITHEQUAL=INVALUE",
	}

	environ := []string{
		"OKTETO_ENV_VAR_1=value1",
		"OKTETO_ENV_VAR_2=value2",
		"NON_OKTETO_ENV_VAR=value3",
		"OKTETO_ENV_VAR_WITHOUT_VALUE_IS_SKIPPED=MYVALUEWITHEQUAL=INVALUE",
		"OKTETO_ENV_VAR_WITHOUT_VALUE_IS_SKIPPED",
	}

	prefixEnvVars := getOktetoPrefixEnvVars(environ)

	for key, value := range expectedEnvVars {
		assert.Equal(t, value, prefixEnvVars[key])
	}
}

func TestFormatEnvVarValueForDocker(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single line",
			input:    "value",
			expected: "\"value\"",
		},
		{
			name:     "multiple lines",
			input:    "line1\nline2\nline3",
			expected: "\"line1\\nline2\\nline3\"",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "\"\"",
		},
		{
			name:     "single newline",
			input:    "\n",
			expected: "\"\\n\"",
		},
		{
			name:     "trailing newline",
			input:    "line1\nline2\n",
			expected: "\"line1\\nline2\\n\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatEnvVarValueForDocker(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
