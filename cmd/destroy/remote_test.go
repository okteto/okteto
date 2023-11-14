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

package destroy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	filesystem "github.com/okteto/okteto/pkg/filesystem/fake"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBuilder struct {
	err           error
	assertOptions func(o *types.BuildOptions)
}

func (f fakeBuilder) Build(_ context.Context, opts *types.BuildOptions) error {
	if f.assertOptions != nil {
		f.assertOptions(opts)
	}
	return f.err
}

func (fakeBuilder) IsV1() bool { return true }

type fakeRegistry struct {
	registry          map[string]fakeImage
	errAddImageByName error
	errAddImageByOpts error
}

// fakeImage represents the data from an image
type fakeImage struct {
	Registry string
	Repo     string
	Tag      string
	ImageRef string
	Args     []string
}

func newFakeRegistry() fakeRegistry {
	return fakeRegistry{
		registry: map[string]fakeImage{},
	}
}

func (fr fakeRegistry) HasGlobalPushAccess() (bool, error) { return false, nil }

func (fr fakeRegistry) GetImageTagWithDigest(imageTag string) (string, error) {
	if _, ok := fr.registry[imageTag]; !ok {
		return "", oktetoErrors.ErrNotFound
	}
	return imageTag, nil
}
func (fr fakeRegistry) IsOktetoRegistry(_ string) bool { return false }

func (fr fakeRegistry) AddImageByName(images ...string) error {
	if fr.errAddImageByName != nil {
		return fr.errAddImageByName
	}
	for _, image := range images {
		fr.registry[image] = fakeImage{}
	}
	return nil
}
func (fr fakeRegistry) AddImageByOpts(opts *types.BuildOptions) error {
	if fr.errAddImageByOpts != nil {
		return fr.errAddImageByOpts
	}
	fr.registry[opts.Tag] = fakeImage{Args: opts.BuildArgs}
	return nil
}

func (fr fakeRegistry) GetImageReference(image string) (registry.OktetoImageReference, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return registry.OktetoImageReference{}, err
	}
	return registry.OktetoImageReference{
		Registry: ref.Context().RegistryStr(),
		Repo:     ref.Context().RepositoryStr(),
		Tag:      ref.Identifier(),
		Image:    image,
	}, nil
}

func (fr fakeRegistry) IsGlobalRegistry(image string) bool { return false }

func (fr fakeRegistry) GetRegistryAndRepo(image string) (string, string) { return "", "" }
func (fr fakeRegistry) GetRepoNameAndTag(repo string) (string, string)   { return "", "" }

func TestRemoteTest(t *testing.T) {
	ctx := context.Background()
	wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/"))
	fs := afero.NewMemMapFs()
	tempCreator := filesystem.NewTemporalDirectoryCtrl(fs)

	type config struct {
		wd            filesystem.FakeWorkingDirectoryCtrlErrors
		tempFsCreator error
		options       *Options
		builderErr    error
	}
	var tests = []struct {
		config   config
		expected error
		name     string
	}{
		{
			name: "OS can't access to the working directory",
			config: config{
				wd: filesystem.FakeWorkingDirectoryCtrlErrors{
					Getter: assert.AnError,
				},
				options: &Options{},
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
				options: &Options{},
			},
			expected: assert.AnError,
		},
		{
			name: "build incorrect",
			config: config{
				options:    &Options{},
				builderErr: assert.AnError,
			},
			expected: oktetoErrors.UserError{
				E: fmt.Errorf("error during destroy of the development environment: %w", assert.AnError),
			},
		},
		{
			name: "build with command error",
			config: config{
				options: &Options{},
				builderErr: build.OktetoCommandErr{
					Stage: "test",
					Err:   assert.AnError,
				},
			},
			expected: oktetoErrors.UserError{
				E: fmt.Errorf("error during development environment deployment: %w", assert.AnError),
			},
		},
		{
			name: "everything correct",
			config: config{
				options: &Options{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wdCtrl.SetErrors(tt.config.wd)
			tempCreator.SetError(tt.config.tempFsCreator)
			rdc := remoteDestroyCommand{
				builder:              fakeBuilder{err: tt.config.builderErr},
				fs:                   fs,
				workingDirectoryCtrl: wdCtrl,
				temporalCtrl:         tempCreator,
				destroyImage:         "",
				registry:             newFakeRegistry(),
				clusterMetadata: func(ctx context.Context) (*types.ClusterMetadata, error) {
					return &types.ClusterMetadata{
						Certificate: []byte("cert"),
						ServerName:  "1.2.3.4:443",
					}, nil
				},
			}
			err := rdc.destroy(ctx, tt.config.options)
			assert.Equal(t, tt.expected, err)
		})
	}
}

func TestGetDestroyFlags(t *testing.T) {
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
				opts: &Options{},
			},
		},
		{
			name: "name set",
			config: config{
				opts: &Options{
					Name: "test",
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "name multiple words",
			config: config{
				opts: &Options{
					Name: "this is a test",
				},
			},
			expected: []string{"--name \"this is a test\""},
		},
		{
			name: "namespace set",
			config: config{
				opts: &Options{
					Namespace: "test",
				},
			},
			expected: []string{"--namespace test"},
		},
		{
			name: "manifest path set",
			config: config{
				opts: &Options{
					ManifestPathFlag: "/hello/this/is/a/test",
				},
			},
			expected: []string{"--file /hello/this/is/a/test"},
		},
		{
			name: "destroy volumes set",
			config: config{
				opts: &Options{
					DestroyVolumes: true,
				},
			},
			expected: []string{"--volumes"},
		},
		{
			name: "force destroy set",
			config: config{
				opts: &Options{
					ForceDestroy: true,
				},
			},
			expected: []string{"--force-destroy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := getDestroyFlags(tt.config.opts)
			assert.Equal(t, tt.expected, flags)
		})
	}
}

func TestCreateDockerfile(t *testing.T) {
	wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/"))
	fs := afero.NewMemMapFs()
	type config struct {
		wd   filesystem.FakeWorkingDirectoryCtrlErrors
		opts *Options
	}
	type expected struct {
		err            error
		dockerfileName string
	}
	var tests = []struct {
		config   config
		expected expected
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
				opts: &Options{},
			},
			expected: expected{
				dockerfileName: filepath.Clean("/test/Dockerfile.destroy"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wdCtrl.SetErrors(tt.config.wd)
			rdc := remoteDestroyCommand{
				fs:                   fs,
				destroyImage:         "test-image",
				workingDirectoryCtrl: wdCtrl,
				registry:             newFakeRegistry(),
			}
			dockerfileName, err := rdc.createDockerfile("/test", tt.config.opts)
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.dockerfileName, dockerfileName)

			if tt.expected.err == nil {
				_, err = rdc.fs.Stat(filepath.Join("/test", dockerfileTemporalNane))
				assert.NoError(t, err)
				content, err := afero.ReadFile(rdc.fs, filepath.Join("/test", dockerfileTemporalNane))
				assert.NoError(t, err)
				assert.True(t, strings.Contains(string(content), fmt.Sprintf("ARG %s", model.OktetoActionNameEnvVar)))
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

func TestRemoteDestroyWithSshAgent(t *testing.T) {
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
	rdc := remoteDestroyCommand{
		sshAuthSockEnvvar:    envvarName,
		knownHostsPath:       knowHostFile.Name(),
		builder:              fakeBuilder{assertOptions: assertFn},
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/")),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
		clusterMetadata: func(context.Context) (*types.ClusterMetadata, error) {
			return &types.ClusterMetadata{}, nil
		},
	}

	assert.NoError(t, rdc.destroy(context.Background(), &Options{}))
}

func TestRemoteDestroyWithBadSshAgent(t *testing.T) {
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

	rdc := remoteDestroyCommand{
		sshAuthSockEnvvar:    envvarName,
		knownHostsPath:       "inexistent-file",
		builder:              fakeBuilder{assertOptions: assertFn},
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/")),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
		clusterMetadata: func(context.Context) (*types.ClusterMetadata, error) {
			return &types.ClusterMetadata{}, nil
		},
	}

	assert.NoError(t, rdc.destroy(context.Background(), &Options{}))
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
