// Copyright 2026 The Okteto Authors
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

package buildkit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/tonistiigi/fsutil"
)

const (
	testRepoURL        = "https://github.com/okteto/movies"
	testRepoID         = "github.com/okteto/movies"
	testContextFolder  = "services/api"
	testDockerfile     = "services/api/Dockerfile"
	testHomeID         = "home"
	testRandomSource   = "random"
	testSourceOpt      = "okteto-local-source:context"
	testFilePermission = 0600
	testDirPermission  = 0700
)

// testBuildContext is a repository in a temporary folder with a build context in "services/api" and the request
// of an image build of "services/api/Dockerfile"
type testBuildContext struct {
	strategy    *smartBuildContext
	root        string
	contextPath string
	// dockerfile is the user's Dockerfile and translated is the copy with a random name that the CLI sends
	dockerfile string
	translated string
	target     string
	outputMode string
}

func newTestBuildContext(t *testing.T) *testBuildContext {
	t.Helper()
	root := t.TempDir()
	contextPath := filepath.Join(root, "services", "api")
	require.NoError(t, os.MkdirAll(contextPath, testDirPermission))
	return &testBuildContext{
		root:        root,
		contextPath: contextPath,
		dockerfile:  filepath.Join(contextPath, "Dockerfile"),
		translated:  filepath.Join(t.TempDir(), "buildkit-1234"),
		strategy: &smartBuildContext{
			fs:          afero.NewOsFs(),
			logger:      io.NewIOController(),
			repoURL:     func(string) string { return testRepoURL },
			repoRoot:    func(string) (string, error) { return root, nil },
			homeID:      func() (string, error) { return testHomeID, nil },
			inInstaller: func() bool { return false },
			inCI:        func() bool { return false },
			randomID:    func() string { return testRandomSource },
		},
	}
}

func (c *testBuildContext) buildOptions() *types.BuildOptions {
	return &types.BuildOptions{
		Path:               c.contextPath,
		File:               c.translated,
		OriginalDockerfile: c.dockerfile,
		Target:             c.target,
		OutputMode:         c.outputMode,
	}
}

// configure runs the strategy on a request that uploads the build context
func (c *testBuildContext) configure(t *testing.T) *client.SolveOpt {
	t.Helper()
	mount, err := fsutil.NewFS(c.contextPath)
	require.NoError(t, err)
	opt := &client.SolveOpt{
		LocalMounts:   map[string]fsutil.FS{"context": mount},
		FrontendAttrs: map[string]string{"filename": "buildkit-1234"},
	}
	c.strategy.configure(opt, c.buildOptions())
	return opt
}

func (c *testBuildContext) expectedSource(t *testing.T) string {
	t.Helper()
	absPath, err := filepath.Abs(c.contextPath)
	require.NoError(t, err)
	resolved, err := filepath.EvalSymlinks(absPath)
	require.NoError(t, err)
	return hashID(testHomeID, resolved)
}

// testKey returns the shared key of a build of the test repository
func testKey(contextFolder, ignoreRules, build, target string) string {
	return "okteto-" + hashID("v1", testRepoID, contextFolder, ignoreRules, build, target)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), testFilePermission))
}

func TestNewBuildContextStrategy(t *testing.T) {
	tests := []struct {
		expected buildContextStrategy
		name     string
		value    string
	}{
		{name: "not set", value: "", expected: noopBuildContext{}},
		{name: "false", value: "false", expected: noopBuildContext{}},
		{name: "true", value: "true", expected: &smartBuildContext{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(SmartBuildContextEnabledEnvVar, tt.value)
			strategy := newBuildContextStrategy(afero.NewMemMapFs(), io.NewIOController())
			require.IsType(t, tt.expected, strategy)
		})
	}
}

func TestNewBuildContextStrategyHasEveryDependency(t *testing.T) {
	t.Setenv(SmartBuildContextEnabledEnvVar, "true")
	fs := afero.NewMemMapFs()
	logger := io.NewIOController()

	strategy, ok := newBuildContextStrategy(fs, logger).(*smartBuildContext)

	require.True(t, ok)
	require.Equal(t, fs, strategy.fs)
	require.Same(t, logger, strategy.logger)
	require.NotNil(t, strategy.repoURL)
	require.NotNil(t, strategy.repoRoot)
	require.NotNil(t, strategy.homeID)
	require.NotNil(t, strategy.inInstaller)
	require.NotNil(t, strategy.inCI)
	require.NotNil(t, strategy.randomID)
}

func TestNewSolveOptBuilderSetsTheBuildContextStrategy(t *testing.T) {
	tests := []struct {
		expected buildContextStrategy
		name     string
		value    string
	}{
		{name: "not set", value: "", expected: noopBuildContext{}},
		{name: "enabled", value: "true", expected: &smartBuildContext{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(SmartBuildContextEnabledEnvVar, tt.value)

			builder, err := NewSolveOptBuilder(nil, nil, &fakeOktetoContext{}, afero.NewMemMapFs(), io.NewIOController(), nil)

			require.NoError(t, err)
			require.IsType(t, tt.expected, builder.buildContext)
		})
	}
}

func TestNoopBuildContextKeepsTheRequest(t *testing.T) {
	opt := &client.SolveOpt{
		LocalMounts:   map[string]fsutil.FS{"context": nil},
		FrontendAttrs: map[string]string{"filename": "Dockerfile"},
	}

	noopBuildContext{}.configure(opt, &types.BuildOptions{Path: t.TempDir()})

	require.Empty(t, opt.SharedKey)
	require.Equal(t, map[string]string{"filename": "Dockerfile"}, opt.FrontendAttrs)
}

func TestSmartBuildContextSetsSharedKeyAndSource(t *testing.T) {
	c := newTestBuildContext(t)

	opt := c.configure(t)

	require.Equal(t, testKey(testContextFolder, "", testDockerfile, ""), opt.SharedKey)
	require.Equal(t, map[string]string{
		"filename":    "buildkit-1234",
		testSourceOpt: c.expectedSource(t),
	}, opt.FrontendAttrs)
}

func TestSmartBuildContextCreatesTheBuildOptionsOfARequestWithoutThem(t *testing.T) {
	c := newTestBuildContext(t)
	mount, err := fsutil.NewFS(c.contextPath)
	require.NoError(t, err)
	opt := &client.SolveOpt{LocalMounts: map[string]fsutil.FS{"context": mount}}

	c.strategy.configure(opt, c.buildOptions())

	require.Equal(t, testKey(testContextFolder, "", testDockerfile, ""), opt.SharedKey)
	require.Equal(t, map[string]string{testSourceOpt: c.expectedSource(t)}, opt.FrontendAttrs)
}

func TestSmartBuildContextWithRelativePaths(t *testing.T) {
	c := newTestBuildContext(t)
	// the root is found from the path that the strategy passes, as the real lookup does
	c.strategy.repoRoot = func(dir string) (string, error) { return filepath.Dir(filepath.Dir(dir)), nil }
	mount, err := fsutil.NewFS(c.contextPath)
	require.NoError(t, err)
	opt := &client.SolveOpt{
		LocalMounts:   map[string]fsutil.FS{"context": mount},
		FrontendAttrs: map[string]string{},
	}
	t.Chdir(c.root)

	c.strategy.configure(opt, &types.BuildOptions{
		Path:               filepath.Join("services", "api"),
		File:               c.translated,
		OriginalDockerfile: filepath.Join("services", "api", "Dockerfile"),
	})

	require.Equal(t, testKey(testContextFolder, "", testDockerfile, ""), opt.SharedKey)
	require.Equal(t, c.expectedSource(t), opt.FrontendAttrs[testSourceOpt])
}

func TestSmartBuildContextOfTwoClonesSharesTheKey(t *testing.T) {
	a := newTestBuildContext(t)
	b := newTestBuildContext(t)

	optA := a.configure(t)
	optB := b.configure(t)

	require.Equal(t, optA.SharedKey, optB.SharedKey)
	require.NotEqual(t, optA.FrontendAttrs[testSourceOpt], optB.FrontendAttrs[testSourceOpt])
}

func TestSmartBuildContextOfTheSameFolderIsStable(t *testing.T) {
	c := newTestBuildContext(t)

	first := c.configure(t)
	second := c.configure(t)

	require.Equal(t, first.SharedKey, second.SharedKey)
	require.Equal(t, first.FrontendAttrs[testSourceOpt], second.FrontendAttrs[testSourceOpt])
}

func TestSmartBuildContextKeyChangesWithTheRepository(t *testing.T) {
	c := newTestBuildContext(t)
	first := c.configure(t)

	c.strategy.repoURL = func(string) string { return "https://github.com/okteto/other" }
	second := c.configure(t)

	require.NotEqual(t, first.SharedKey, second.SharedKey)
}

func TestSmartBuildContextKeyChangesWithTheFolder(t *testing.T) {
	c := newTestBuildContext(t)
	first := c.configure(t)

	c.contextPath = filepath.Join(c.root, "services")
	second := c.configure(t)

	require.Equal(t, testKey("services", "", testDockerfile, ""), second.SharedKey)
	require.NotEqual(t, first.SharedKey, second.SharedKey)
}

func TestSmartBuildContextKeyOfTheRepositoryRoot(t *testing.T) {
	c := newTestBuildContext(t)
	c.contextPath = c.root

	opt := c.configure(t)

	require.Equal(t, testKey(".", "", testDockerfile, ""), opt.SharedKey)
}

func TestSmartBuildContextKeyChangesWithTheDockerfile(t *testing.T) {
	c := newTestBuildContext(t)
	first := c.configure(t)

	c.dockerfile = filepath.Join(c.contextPath, "Dockerfile.worker")
	second := c.configure(t)

	require.Equal(t, testKey(testContextFolder, "", "services/api/Dockerfile.worker", ""), second.SharedKey)
	require.NotEqual(t, first.SharedKey, second.SharedKey)
}

func TestSmartBuildContextKeyChangesWithTheTarget(t *testing.T) {
	c := newTestBuildContext(t)
	first := c.configure(t)

	c.target = "dev"
	second := c.configure(t)

	require.Equal(t, testKey(testContextFolder, "", testDockerfile, "dev"), second.SharedKey)
	require.NotEqual(t, first.SharedKey, second.SharedKey)
}

func TestSmartBuildContextKeyDoesNotChangeWithTheTranslatedDockerfile(t *testing.T) {
	c := newTestBuildContext(t)
	first := c.configure(t)

	c.translated = filepath.Join(t.TempDir(), "buildkit-5678")
	second := c.configure(t)

	require.Equal(t, first.SharedKey, second.SharedKey)
}

func TestSmartBuildContextKeyOfADockerfileOutsideTheRepository(t *testing.T) {
	c := newTestBuildContext(t)
	c.dockerfile = filepath.Join(t.TempDir(), "Dockerfile.shared")
	writeTestFile(t, c.dockerfile, "FROM busybox\nCOPY . /app\n")

	opt := c.configure(t)

	build := "content:" + hashID("FROM busybox\nCOPY . /app\n")
	require.Equal(t, testKey(testContextFolder, "", build, ""), opt.SharedKey)
}

func TestSmartBuildContextKeyOfGeneratedDockerfilesWithTheSameContent(t *testing.T) {
	// the CLI writes the Dockerfile of a compose service with volumes to a file with a random name on every run
	c := newTestBuildContext(t)
	generated := t.TempDir()
	c.dockerfile = filepath.Join(generated, "buildkit-581652291")
	writeTestFile(t, c.dockerfile, "FROM busybox\nCOPY data /data\n")
	first := c.configure(t)

	c.dockerfile = filepath.Join(generated, "buildkit-179283910")
	writeTestFile(t, c.dockerfile, "FROM busybox\nCOPY data /data\n")
	second := c.configure(t)

	require.NotEmpty(t, first.SharedKey)
	require.Equal(t, first.SharedKey, second.SharedKey)
}

func TestSmartBuildContextKeyChangesWithTheContentOfADockerfileOutsideTheRepository(t *testing.T) {
	c := newTestBuildContext(t)
	c.dockerfile = filepath.Join(t.TempDir(), "buildkit-581652291")
	writeTestFile(t, c.dockerfile, "FROM busybox\nCOPY data /data\n")
	first := c.configure(t)

	writeTestFile(t, c.dockerfile, "FROM busybox\nCOPY logs /logs\n")
	second := c.configure(t)

	require.NotEmpty(t, second.SharedKey)
	require.NotEqual(t, first.SharedKey, second.SharedKey)
}

func TestSmartBuildContextKeyOfADockerfileOutsideTheRepositoryThatCanNotBeRead(t *testing.T) {
	c := newTestBuildContext(t)
	c.dockerfile = filepath.Join(t.TempDir(), "Dockerfile.missing")

	opt := c.configure(t)

	require.Equal(t, testKey(testContextFolder, "", "Dockerfile.missing", ""), opt.SharedKey)
}

func TestSmartBuildContextKeyUsesTheFileWithoutOriginalDockerfile(t *testing.T) {
	c := newTestBuildContext(t)
	c.dockerfile = ""
	c.translated = filepath.Join(c.contextPath, "Dockerfile.dev")

	opt := c.configure(t)

	require.Equal(t, testKey(testContextFolder, "", "services/api/Dockerfile.dev", ""), opt.SharedKey)
}

func TestSmartBuildContextKeyOfRemoteOperations(t *testing.T) {
	tests := []struct {
		name       string
		outputMode string
		dockerfile string
	}{
		{name: "deploy", outputMode: "deploy", dockerfile: "Dockerfile.deploy"},
		{name: "destroy", outputMode: "destroy", dockerfile: "Dockerfile.destroy"},
		{name: "test", outputMode: "test", dockerfile: "Dockerfile.test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestBuildContext(t)
			c.outputMode = tt.outputMode
			// the CLI generates the Dockerfile of a remote operation in a temporary folder
			c.dockerfile = filepath.Join(t.TempDir(), tt.dockerfile)

			opt := c.configure(t)

			require.Equal(t, testKey(testContextFolder, "", "okteto-remote-operation", ""), opt.SharedKey)
		})
	}
}

func TestSmartBuildContextKeyOfAnImageBuildIsNotTheOneOfARemoteOperation(t *testing.T) {
	c := newTestBuildContext(t)
	image := c.configure(t)

	c.outputMode = "deploy"
	remote := c.configure(t)

	require.NotEqual(t, image.SharedKey, remote.SharedKey)
}

func TestSmartBuildContextKeyUsesTheIgnoreRulesOfTheContext(t *testing.T) {
	c := newTestBuildContext(t)
	writeTestFile(t, filepath.Join(c.contextPath, ".dockerignore"), "node_modules\n.git\n")

	opt := c.configure(t)

	require.Equal(t, testKey(testContextFolder, "node_modules\n.git", testDockerfile, ""), opt.SharedKey)
}

func TestSmartBuildContextKeyPrefersTheIgnoreRulesOfTheDockerfile(t *testing.T) {
	c := newTestBuildContext(t)
	writeTestFile(t, filepath.Join(c.contextPath, ".dockerignore"), "node_modules\n")
	writeTestFile(t, c.translated+".dockerignore", "bin\n")

	opt := c.configure(t)

	require.Equal(t, testKey(testContextFolder, "bin", testDockerfile, ""), opt.SharedKey)
}

func TestSmartBuildContextKeyUsesTheIgnoreRulesOfTheContextWhenTheOnesOfTheDockerfileCanNotBeRead(t *testing.T) {
	c := newTestBuildContext(t)
	require.NoError(t, os.Mkdir(c.translated+".dockerignore", testDirPermission))
	writeTestFile(t, filepath.Join(c.contextPath, ".dockerignore"), "bin\n")

	opt := c.configure(t)

	require.Equal(t, testKey(testContextFolder, "bin", testDockerfile, ""), opt.SharedKey)
}

func TestSmartBuildContextKeyWithoutDockerfile(t *testing.T) {
	c := newTestBuildContext(t)
	c.dockerfile = ""
	c.translated = ""
	writeTestFile(t, filepath.Join(c.contextPath, ".dockerignore"), "node_modules\n")
	// an ignore file in the working directory is not the one of the build context
	t.Chdir(t.TempDir())
	writeTestFile(t, ".dockerignore", "bin\n")

	opt := c.configure(t)

	require.Equal(t, testKey(testContextFolder, "node_modules", "", ""), opt.SharedKey)
}

func TestSmartBuildContextKeyIgnoresCommentsOfTheIgnoreFile(t *testing.T) {
	c := newTestBuildContext(t)
	writeTestFile(t, filepath.Join(c.contextPath, ".dockerignore"), "node_modules\n")
	first := c.configure(t)

	writeTestFile(t, filepath.Join(c.contextPath, ".dockerignore"), "# dependencies\n\nnode_modules\n")
	second := c.configure(t)

	require.Equal(t, first.SharedKey, second.SharedKey)
}

func TestSmartBuildContextSourceIsNewOnEveryBuild(t *testing.T) {
	tests := []struct {
		homeID      func() (string, error)
		name        string
		inInstaller bool
		inCI        bool
	}{
		{
			name:        "in the installer",
			inInstaller: true,
			homeID:      func() (string, error) { return testHomeID, nil },
		},
		{
			name:   "in CI",
			inCI:   true,
			homeID: func() (string, error) { return testHomeID, nil },
		},
		{
			name:   "ID of the home not available",
			homeID: func() (string, error) { return "", errors.New("read-only file system") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestBuildContext(t)
			c.strategy.inInstaller = func() bool { return tt.inInstaller }
			c.strategy.inCI = func() bool { return tt.inCI }
			c.strategy.homeID = tt.homeID
			builds := 0
			c.strategy.randomID = func() string {
				builds++
				return fmt.Sprintf("random-%d", builds)
			}

			first := c.configure(t)
			second := c.configure(t)

			require.NotEmpty(t, first.SharedKey)
			require.Equal(t, first.SharedKey, second.SharedKey)
			require.Equal(t, "random-1", first.FrontendAttrs[testSourceOpt])
			require.Equal(t, "random-2", second.FrontendAttrs[testSourceOpt])
		})
	}
}

func TestSmartBuildContextDoesNotReadTheIDOfTheHomeInTheInstallerAndInCI(t *testing.T) {
	tests := []struct {
		name        string
		inInstaller bool
		inCI        bool
	}{
		{name: "in the installer", inInstaller: true},
		{name: "in CI", inCI: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestBuildContext(t)
			c.strategy.inInstaller = func() bool { return tt.inInstaller }
			c.strategy.inCI = func() bool { return tt.inCI }
			reads := 0
			c.strategy.homeID = func() (string, error) {
				reads++
				return testHomeID, nil
			}

			opt := c.configure(t)

			require.Equal(t, testRandomSource, opt.FrontendAttrs[testSourceOpt])
			require.Zero(t, reads)
		})
	}
}

func TestSmartBuildContextSourceChangesWithTheHome(t *testing.T) {
	c := newTestBuildContext(t)
	first := c.configure(t)

	c.strategy.homeID = func() (string, error) { return "another home", nil }
	second := c.configure(t)

	require.Equal(t, first.SharedKey, second.SharedKey)
	require.NotEqual(t, first.FrontendAttrs[testSourceOpt], second.FrontendAttrs[testSourceOpt])
}

func TestSmartBuildContextKeepsTheRequest(t *testing.T) {
	tests := []struct {
		setup func(c *testBuildContext)
		name  string
	}{
		{
			name:  "not in a git repository with a remote",
			setup: func(c *testBuildContext) { c.strategy.repoURL = func(string) string { return "" } },
		},
		{
			name: "remote that is a local path",
			setup: func(c *testBuildContext) {
				c.strategy.repoURL = func(string) string { return "file:///srv/git/movies" }
			},
		},
		{
			name: "root of the repository not found",
			setup: func(c *testBuildContext) {
				c.strategy.repoRoot = func(string) (string, error) { return "", errors.New("not found") }
			},
		},
		{
			name: "build context outside the repository",
			setup: func(c *testBuildContext) {
				c.strategy.repoRoot = func(string) (string, error) { return filepath.Join(c.root, "other"), nil }
			},
		},
		{
			name: "build context is the parent of the repository",
			setup: func(c *testBuildContext) {
				c.strategy.repoRoot = func(string) (string, error) { return filepath.Join(c.contextPath, "repository"), nil }
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestBuildContext(t)
			tt.setup(c)

			opt := c.configure(t)

			require.Empty(t, opt.SharedKey)
			require.Equal(t, map[string]string{"filename": "buildkit-1234"}, opt.FrontendAttrs)
		})
	}
}

func TestSmartBuildContextKeepsTheRequestWithoutLocalContext(t *testing.T) {
	c := newTestBuildContext(t)
	opt := &client.SolveOpt{FrontendAttrs: map[string]string{"context": "https://github.com/okteto/movies.git"}}

	c.strategy.configure(opt, &types.BuildOptions{Path: "https://github.com/okteto/movies.git"})

	require.Empty(t, opt.SharedKey)
	require.Equal(t, map[string]string{"context": "https://github.com/okteto/movies.git"}, opt.FrontendAttrs)
}

// newTestGitRepository creates a git repository with the given remote and a build context in "services/api"
func newTestGitRepository(t *testing.T, remote string) string {
	t.Helper()
	root := t.TempDir()
	repo, err := git.PlainInit(root, false)
	require.NoError(t, err)
	_, err = repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{remote}})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "services", "api"), testDirPermission))
	return root
}

// configureRealBuildContext runs the strategy that the CLI uses, with its real lookups, as an installer job
func configureRealBuildContext(t *testing.T, contextPath string) *client.SolveOpt {
	t.Helper()
	t.Setenv("OKTETO_IN_INSTALLER", "true")
	// an installer job does not use the Okteto home: this one keeps the test away from the home of the developer
	t.Setenv(constants.OktetoFolderEnvVar, t.TempDir())
	return runRealBuildContext(t, contextPath)
}

// configureRealBuildContextOfAUser runs the strategy that the CLI uses, with its real lookups, as a user with the
// given Okteto home
func configureRealBuildContextOfAUser(t *testing.T, contextPath, oktetoHome string) *client.SolveOpt {
	t.Helper()
	t.Setenv("OKTETO_IN_INSTALLER", "")
	t.Setenv("CI", "")
	t.Setenv(constants.OktetoFolderEnvVar, oktetoHome)
	return runRealBuildContext(t, contextPath)
}

func runRealBuildContext(t *testing.T, contextPath string) *client.SolveOpt {
	t.Helper()
	t.Setenv(SmartBuildContextEnabledEnvVar, "true")
	mount, err := fsutil.NewFS(contextPath)
	require.NoError(t, err)
	opt := &client.SolveOpt{
		LocalMounts:   map[string]fsutil.FS{"context": mount},
		FrontendAttrs: map[string]string{},
	}
	strategy := newBuildContextStrategy(afero.NewOsFs(), io.NewIOController())
	strategy.configure(opt, &types.BuildOptions{Path: contextPath, File: filepath.Join(contextPath, "Dockerfile")})
	return opt
}

func TestSmartBuildContextWithGitRepository(t *testing.T) {
	root := newTestGitRepository(t, "git@github.com:okteto/movies.git")

	opt := configureRealBuildContext(t, filepath.Join(root, "services", "api"))

	require.Equal(t, testKey(testContextFolder, "", testDockerfile, ""), opt.SharedKey)
	require.Len(t, opt.FrontendAttrs[testSourceOpt], 32)
}

func TestSmartBuildContextOfRepositoriesWithRemotesThatDifferAfterASpace(t *testing.T) {
	first := newTestGitRepository(t, "git@ssh.dev.azure.com:v3/acme/My%20Project/RepoA")
	second := newTestGitRepository(t, "git@ssh.dev.azure.com:v3/acme/My%20Project/RepoB")

	firstOpt := configureRealBuildContext(t, filepath.Join(first, "services", "api"))
	secondOpt := configureRealBuildContext(t, filepath.Join(second, "services", "api"))

	require.NotEmpty(t, firstOpt.SharedKey)
	require.NotEmpty(t, secondOpt.SharedKey)
	require.NotEqual(t, firstOpt.SharedKey, secondOpt.SharedKey)
}

func TestSmartBuildContextIsNotUsedWithALocalRemote(t *testing.T) {
	root := newTestGitRepository(t, filepath.Join(t.TempDir(), "origin.git"))

	opt := configureRealBuildContext(t, filepath.Join(root, "services", "api"))

	require.Empty(t, opt.SharedKey)
	require.Empty(t, opt.FrontendAttrs)
}

func TestSmartBuildContextSourceInCI(t *testing.T) {
	home := t.TempDir()
	t.Setenv(SmartBuildContextEnabledEnvVar, "true")
	t.Setenv("OKTETO_IN_INSTALLER", "")
	t.Setenv("CI", "true")
	t.Setenv(constants.OktetoFolderEnvVar, home)
	c := newTestBuildContext(t)
	strategy, ok := newBuildContextStrategy(afero.NewOsFs(), io.NewIOController()).(*smartBuildContext)
	require.True(t, ok)

	first := strategy.source(c.contextPath)
	second := strategy.source(c.contextPath)

	require.Len(t, first, 32)
	require.NotEqual(t, first, second)
	require.NoFileExists(t, filepath.Join(home, sourceIDFile))
}

func TestSmartBuildContextSourceOfAUser(t *testing.T) {
	root := newTestGitRepository(t, "git@github.com:okteto/movies.git")
	contextPath := filepath.Join(root, "services", "api")
	home := t.TempDir()

	first := configureRealBuildContextOfAUser(t, contextPath, home)
	second := configureRealBuildContextOfAUser(t, contextPath, home)

	require.Len(t, first.FrontendAttrs[testSourceOpt], 32)
	require.Equal(t, first.FrontendAttrs[testSourceOpt], second.FrontendAttrs[testSourceOpt])
	require.FileExists(t, filepath.Join(home, sourceIDFile))
}

// two containers of the same image build the same path, each one with its own home
func TestSmartBuildContextSourceOfAUserWithANewHome(t *testing.T) {
	root := newTestGitRepository(t, "git@github.com:okteto/movies.git")
	contextPath := filepath.Join(root, "services", "api")

	first := configureRealBuildContextOfAUser(t, contextPath, t.TempDir())
	second := configureRealBuildContextOfAUser(t, contextPath, t.TempDir())

	require.NotEmpty(t, first.SharedKey)
	require.Equal(t, first.SharedKey, second.SharedKey)
	require.NotEqual(t, first.FrontendAttrs[testSourceOpt], second.FrontendAttrs[testSourceOpt])
}

func TestSmartBuildContextSourceOfAUserWithAHomeThatCanNotBeWritten(t *testing.T) {
	t.Setenv(SmartBuildContextEnabledEnvVar, "true")
	t.Setenv("OKTETO_IN_INSTALLER", "")
	t.Setenv("CI", "")
	home := t.TempDir()
	t.Setenv(constants.OktetoFolderEnvVar, home)
	c := newTestBuildContext(t)
	fs := afero.NewReadOnlyFs(afero.NewOsFs())
	strategy, ok := newBuildContextStrategy(fs, io.NewIOController()).(*smartBuildContext)
	require.True(t, ok)

	first := strategy.source(c.contextPath)
	second := strategy.source(c.contextPath)

	require.Len(t, first, 32)
	require.NotEqual(t, first, second)
	require.NoFileExists(t, filepath.Join(home, sourceIDFile))
}

func TestHomeSourceIDIsCreatedOnce(t *testing.T) {
	fs := afero.NewOsFs()
	home := t.TempDir()

	first, err := homeSourceID(fs, home)
	require.NoError(t, err)
	second, err := homeSourceID(fs, home)
	require.NoError(t, err)

	require.Len(t, first, 32)
	require.Equal(t, first, second)
	saved, err := os.ReadFile(filepath.Join(home, sourceIDFile))
	require.NoError(t, err)
	require.Equal(t, first, string(saved))
	// the file that is renamed is not left behind
	entries, err := os.ReadDir(home)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestHomeSourceIDOfAnotherHome(t *testing.T) {
	fs := afero.NewOsFs()

	first, err := homeSourceID(fs, t.TempDir())
	require.NoError(t, err)
	second, err := homeSourceID(fs, t.TempDir())
	require.NoError(t, err)

	require.NotEqual(t, first, second)
}

func TestHomeSourceIDKeepsTheValueOfTheFile(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, sourceIDFile), "saved by another build\n")

	// a home that can only be read works with the value that it has
	id, err := homeSourceID(afero.NewReadOnlyFs(afero.NewOsFs()), home)

	require.NoError(t, err)
	require.Equal(t, "saved by another build", id)
}

func TestHomeSourceIDReplacesAFileWithoutValue(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "empty file", content: ""},
		{name: "blank file", content: " \n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			writeTestFile(t, filepath.Join(home, sourceIDFile), tt.content)

			id, err := homeSourceID(afero.NewOsFs(), home)

			require.NoError(t, err)
			require.Len(t, id, 32)
			saved, err := os.ReadFile(filepath.Join(home, sourceIDFile))
			require.NoError(t, err)
			require.Equal(t, id, string(saved))
		})
	}
}

func TestHomeSourceIDFailsWhenTheHomeCanNotBeWritten(t *testing.T) {
	home := t.TempDir()

	id, err := homeSourceID(afero.NewReadOnlyFs(afero.NewOsFs()), home)

	require.Error(t, err)
	require.Empty(t, id)
	require.NoFileExists(t, filepath.Join(home, sourceIDFile))
}

// renameFailsFs is a filesystem that creates files and can not rename them
type renameFailsFs struct {
	afero.Fs
}

func (renameFailsFs) Rename(string, string) error {
	return errors.New("rename not supported")
}

func TestHomeSourceIDFailsWhenTheFileCanNotBeRenamed(t *testing.T) {
	home := t.TempDir()

	id, err := homeSourceID(renameFailsFs{afero.NewOsFs()}, home)

	require.EqualError(t, err, "rename not supported")
	require.Empty(t, id)
	entries, err := os.ReadDir(home)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestHomeSourceIDIsTheSameForBuildsThatStartTogether(t *testing.T) {
	const builds = 16
	fs := afero.NewOsFs()
	home := t.TempDir()
	ids := make([]string, builds)
	errs := make([]error, builds)
	var wg sync.WaitGroup
	for i := range builds {
		wg.Go(func() {
			ids[i], errs[i] = homeSourceID(fs, home)
		})
	}
	wg.Wait()

	for i := range builds {
		require.NoError(t, errs[i])
		require.Equal(t, ids[0], ids[i])
	}
}

func TestPathInRepository(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "services", "api"), testDirPermission))
	tests := []struct {
		name     string
		path     string
		expected string
		inside   bool
	}{
		{name: "folder", path: filepath.Join(root, "services", "api"), expected: "services/api", inside: true},
		{name: "file that does not exist", path: filepath.Join(root, "services", "api", "Dockerfile"), expected: "services/api/Dockerfile", inside: true},
		{name: "root", path: root, expected: ".", inside: true},
		{name: "parent of the root", path: filepath.Dir(root)},
		{name: "folder next to the root with its name as prefix", path: root + "-other"},
		{name: "path that goes out of the root", path: filepath.Join(root, "..", "other")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relPath, inside := pathInRepository(root, tt.path)

			require.Equal(t, tt.inside, inside)
			require.Equal(t, tt.expected, relPath)
		})
	}
}

func TestPathInRepositoryWithARootThatIsNotAbsolute(t *testing.T) {
	relPath, inside := pathInRepository(filepath.Join("relative", "root"), t.TempDir())

	require.False(t, inside)
	require.Empty(t, relPath)
}

func TestResolveSymlinksKeepsAPathWithoutExistingParts(t *testing.T) {
	p := filepath.Join("does-not-exist", "folder", "file")

	require.Equal(t, p, resolveSymlinks(p))
}

func TestRepositoryID(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected string
	}{
		{name: "https", repoURL: "https://github.com/okteto/movies", expected: "github.com/okteto/movies"},
		{name: "port", repoURL: "https://git.example.com:2222/okteto/movies", expected: "git.example.com/okteto/movies"},
		{name: "host in upper case", repoURL: "https://GitHub.com/okteto/movies", expected: "github.com/okteto/movies"},
		{name: "trailing slash", repoURL: "https://github.com/okteto/movies/", expected: "github.com/okteto/movies"},
		{name: "git suffix", repoURL: "git://github.com/okteto/movies.git", expected: "github.com/okteto/movies"},
		{name: "subgroups", repoURL: "https://gitlab.com/okteto/samples/movies", expected: "gitlab.com/okteto/samples/movies"},
		{name: "no repository", repoURL: "", expected: ""},
		{name: "local path", repoURL: "file:///srv/git/movies", expected: ""},
		{name: "relative local path", repoURL: "file://../movies", expected: ""},
		{name: "host without path", repoURL: "https://github.com/", expected: ""},
		{name: "not a url", repoURL: "https://github.com/%zz", expected: ""},
		{name: "path without scheme", repoURL: "/srv/git/movies", expected: ""},
		{name: "relative path without scheme", repoURL: "okteto/movies", expected: ""},
		// what the CLI returns for remotes whose path has characters that its parser does not expect
		{name: "rest of the path in the query", repoURL: "https://ssh.dev.azure.com/v3/acme/My?%20Project/RepoA", expected: "ssh.dev.azure.com/v3/acme/My?%20Project/RepoA"},
		{name: "another repository with the same path before the query", repoURL: "https://ssh.dev.azure.com/v3/acme/My?%20Project/RepoB", expected: "ssh.dev.azure.com/v3/acme/My?%20Project/RepoB"},
		{name: "query and fragment", repoURL: "https://github.com/okteto/movies?ref=x#frag", expected: "github.com/okteto/movies?ref=x#frag"},
		{name: "remote with a home folder", repoURL: "https://file//git?@host.example.com:~user/repo.git", expected: ""},
		{name: "windows path with backslashes", repoURL: "https://file//C?:%5Crepos%5Corigin.git", expected: ""},
		{name: "windows path with slashes", repoURL: "https://C/repos/origin", expected: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, repositoryID(tt.repoURL))
		})
	}
}

func TestIgnorePatterns(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{name: "empty", content: "", expected: nil},
		{name: "patterns", content: "node_modules\n.git\n", expected: []string{"node_modules", ".git"}},
		{name: "comments and empty lines", content: "# dependencies\n\n  node_modules  \n\t\n", expected: []string{"node_modules"}},
		{name: "windows line endings", content: "node_modules\r\n.git\r\n", expected: []string{"node_modules", ".git"}},
		{name: "byte order mark", content: "\xef\xbb\xbfnode_modules\n", expected: []string{"node_modules"}},
		{name: "backslashes are kept on every system", content: "bin\\Debug\n", expected: []string{"bin\\Debug"}},
		{name: "exceptions", content: "*\n!src\n", expected: []string{"*", "!src"}},
		{name: "number sign after spaces is a pattern", content: "node_modules\n  #secret\n", expected: []string{"node_modules", "#secret"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, ignorePatterns([]byte(tt.content)))
		})
	}
}

func TestIgnorePatternsOfAVeryLongLine(t *testing.T) {
	// longer than the 64 KiB that a scanner accepts by default
	pattern := strings.Repeat("a", 70000)

	patterns := ignorePatterns([]byte("first\n" + pattern + "\nlast\n"))

	require.Equal(t, []string{"first", pattern, "last"}, patterns)
}

func TestRunningInCI(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{name: "not set", value: "", expected: false},
		{name: "true", value: "true", expected: true},
		{name: "one", value: "1", expected: true},
		{name: "false", value: "false", expected: false},
		{name: "zero", value: "0", expected: false},
		{name: "name of the system", value: "woodpecker", expected: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CI", tt.value)

			require.Equal(t, tt.expected, runningInCI())
		})
	}
}

func TestIsRemoteOperation(t *testing.T) {
	tests := []struct {
		name       string
		outputMode string
		expected   bool
	}{
		{name: "deploy", outputMode: "deploy", expected: true},
		{name: "destroy", outputMode: "destroy", expected: true},
		{name: "test", outputMode: "test", expected: true},
		{name: "image build", outputMode: "", expected: false},
		{name: "progress of an image build", outputMode: "plain", expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, isRemoteOperation(tt.outputMode))
		})
	}
}

func TestHashID(t *testing.T) {
	require.Len(t, hashID("a"), 32)
	require.Equal(t, hashID("a", "b"), hashID("a", "b"))
	require.NotEqual(t, hashID("ab", "c"), hashID("a", "bc"))
	require.NotEqual(t, hashID("a"), hashID("a", ""))
}

func TestRandomSource(t *testing.T) {
	first := randomSource()
	second := randomSource()

	require.Len(t, first, 32)
	require.NotEqual(t, first, second)
}
