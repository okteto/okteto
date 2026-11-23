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
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/denisbrodbeck/machineid"
	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/repository"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

const (
	// SmartBuildContextEnabledEnvVar makes BuildKit reuse the build context that it saved for the same
	// repository, whoever sent it: another folder, another user or a pipeline.
	// Disabled by default: it needs a BuildKit that understands the build option that names the source of the
	// files, and without the variable the CLI sends exactly what it sent before.
	SmartBuildContextEnabledEnvVar = "OKTETO_BUILDKIT_SMART_BUILD_CONTEXT_ENABLED"

	// contextLocalName is the name of the local directory with the build context in a BuildKit request
	contextLocalName = "context"

	// localSourceOptPrefix is the prefix of the build option that tells Okteto's BuildKit which directory the
	// files of a local directory come from: "okteto-local-source:<local name>=<source>". BuildKit compares the
	// files by modification time when the copy that it saved was sent from the same source, and by content when
	// it was sent from another one.
	localSourceOptPrefix = "okteto-local-source:"

	// buildContextKeyVersion changes when the values that make the shared key change
	buildContextKeyVersion = "v1"

	// remoteOperationBuild is the value that identifies the Dockerfile of a remote deploy, destroy or test in the
	// shared key. The CLI generates it and it always copies the whole build context, so the three operations can
	// reuse the same saved copy.
	remoteOperationBuild = "okteto-remote-operation"

	// dockerfileContentPrefix marks a Dockerfile that the shared key identifies by its content
	dockerfileContentPrefix = "content:"

	buildContextIDLength = 32
	randomSourceBytes    = 16
)

// buildContextStrategy sets how BuildKit reuses the build context that it saved in previous builds
type buildContextStrategy interface {
	configure(opt *client.SolveOpt, buildOptions *types.BuildOptions)
}

// newBuildContextStrategy reads the feature flag once and returns the strategy to use
func newBuildContextStrategy(fs afero.Fs, logger *io.Controller) buildContextStrategy {
	if !env.LoadBoolean(SmartBuildContextEnabledEnvVar) {
		return noopBuildContext{}
	}
	return &smartBuildContext{
		fs:          fs,
		logger:      logger,
		repoURL:     func(dir string) string { return repository.NewRepository(dir).GetAnonymizedRepo() },
		repoRoot:    repository.FindTopLevelGitDir,
		machineID:   machineid.ID,
		inInstaller: config.RunningInInstaller,
		inCI:        runningInCI,
		randomID:    randomSource,
	}
}

// runningInCI reports whether the CI variable is set to something other than false: most systems set it to "true"
// and some to their name
func runningInCI() bool {
	value := os.Getenv(constants.CIEnvVar)
	if value == "" {
		return false
	}
	enabled, err := strconv.ParseBool(value)
	return err != nil || enabled
}

// noopBuildContext keeps the request as it is: BuildKit reuses one saved copy for all the builds that it receives
// and compares the files by modification time
type noopBuildContext struct{}

func (noopBuildContext) configure(*client.SolveOpt, *types.BuildOptions) {}

// smartBuildContext gives BuildKit a key that is the same for everybody who runs the same build of the same
// repository (same folder, ignore rules, Dockerfile and target), and the source of the files that are sent
type smartBuildContext struct {
	fs          afero.Fs
	logger      *io.Controller
	repoURL     func(dir string) string
	repoRoot    func(dir string) (string, error)
	machineID   func() (string, error)
	inInstaller func() bool
	inCI        func() bool
	randomID    func() string
}

func (s *smartBuildContext) configure(opt *client.SolveOpt, buildOptions *types.BuildOptions) {
	if _, ok := opt.LocalMounts[contextLocalName]; !ok {
		// the build context is not a local directory, so nothing is uploaded
		return
	}
	contextPath, err := uploadedFolder(buildOptions.Path)
	if err != nil {
		s.logger.Infof("smart build context not used: invalid build context '%s': %s", buildOptions.Path, err)
		return
	}
	key := s.sharedKey(contextPath, buildOptions)
	if key == "" {
		return
	}
	source := s.source(contextPath)
	s.logger.Infof("smart build context: shared key '%s', source '%s'", key, source)

	opt.SharedKey = key
	if opt.FrontendAttrs == nil {
		opt.FrontendAttrs = map[string]string{}
	}
	opt.FrontendAttrs[localSourceOptPrefix+contextLocalName] = source
}

// uploadedFolder returns the folder that is uploaded for a build context. BuildKit's client resolves the symbolic
// links of the path before it walks it, so the repository, the key and the source are the ones of that folder.
func uploadedFolder(contextPath string) (string, error) {
	// before filepath.Abs, which removes ".." without looking at the links before it
	if resolved, err := filepath.EvalSymlinks(contextPath); err == nil {
		contextPath = resolved
	}
	absPath, err := filepath.Abs(contextPath)
	if err != nil {
		return "", err
	}
	// the working directory of a relative path can have links too
	return resolveSymlinks(absPath), nil
}

// sharedKey returns the key of the copy that BuildKit saves for the build context. It only depends on what
// decides which files are sent: the repository, the folder inside the repository, the ignore rules and the
// Dockerfile with its target, because BuildKit only keeps the paths that the build copies.
// It returns "" when the build context is not inside a git repository with a remote.
func (s *smartBuildContext) sharedKey(contextPath string, buildOptions *types.BuildOptions) string {
	repo := repositoryID(s.repoURL(contextPath))
	if repo == "" {
		s.logger.Infof("smart build context not used: '%s' is not in a git repository with a remote host", contextPath)
		return ""
	}
	root, err := s.repoRoot(contextPath)
	if err != nil {
		s.logger.Infof("smart build context not used: could not find the root of the repository of '%s': %s", contextPath, err)
		return ""
	}
	relPath, ok := pathInRepository(root, contextPath)
	if !ok {
		s.logger.Infof("smart build context not used: '%s' is not inside the repository '%s'", contextPath, root)
		return ""
	}
	rules := strings.Join(s.ignoreRules(contextPath, buildOptions.File), "\n")
	build := s.buildID(root, buildOptions)
	return "okteto-" + hashID(buildContextKeyVersion, repo, relPath, rules, build, buildOptions.Target)
}

// pathInRepository returns the path of p inside the repository, with slashes, and false when p is outside of it.
// Symbolic links are resolved in both: the root of the repository comes without them, and the same folder has to
// give the same path however it was reached.
func pathInRepository(repoRoot, p string) (string, bool) {
	relPath, err := filepath.Rel(resolveSymlinks(repoRoot), resolveSymlinks(p))
	if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(relPath), true
}

// resolveSymlinks returns p without symbolic links. When the last elements of p do not exist, it resolves the part
// of p that does.
func resolveSymlinks(p string) string {
	missing := ""
	for existing := p; ; {
		if resolved, err := filepath.EvalSymlinks(existing); err == nil {
			return filepath.Join(resolved, missing)
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return p
		}
		missing = filepath.Join(filepath.Base(existing), missing)
		existing = parent
	}
}

// buildID identifies the Dockerfile of a build in the shared key. Inside the repository it is its path, so the key
// stays the same when the Dockerfile is edited. Outside of it the path is another one on every machine, and on every
// run for the Dockerfiles that the CLI generates, so it is its content.
func (s *smartBuildContext) buildID(repoRoot string, buildOptions *types.BuildOptions) string {
	if isRemoteOperation(buildOptions.OutputMode) {
		return remoteOperationBuild
	}
	// File is a translated copy with a random name when OriginalDockerfile is set
	dockerfile := buildOptions.OriginalDockerfile
	if dockerfile == "" {
		dockerfile = buildOptions.File
	}
	if dockerfile == "" {
		return ""
	}
	if dockerfilePath, err := filepath.Abs(dockerfile); err == nil {
		if relPath, ok := pathInRepository(repoRoot, dockerfilePath); ok {
			return relPath
		}
	}
	content, err := afero.ReadFile(s.fs, dockerfile)
	if err != nil {
		return filepath.Base(dockerfile)
	}
	return dockerfileContentPrefix + hashID(string(content))
}

// isRemoteOperation reports whether the build runs a remote deploy, destroy or test. These are the output modes
// that pkg/cmd/build sets for them.
func isRemoteOperation(outputMode string) bool {
	switch outputMode {
	case "deploy", "destroy", "test":
		return true
	default:
		return false
	}
}

// repositoryID returns the host and the path of a repository URL, so that the clones of a repository get the same
// value whatever protocol, port or user they were cloned with. It returns "" for a remote that is a local path,
// which does not identify the repository outside the machine.
func repositoryID(repoURL string) string {
	if repoURL == "" {
		return ""
	}
	u, err := url.Parse(repoURL)
	if err != nil || u.Scheme == "file" {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	// the URL that the CLI builds for a remote without a host has "file" or the letter of a Windows drive in its place
	if host == "" || host == "file" || len(host) == 1 {
		return ""
	}
	repoPath := strings.Trim(strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git"), "/")
	if repoPath == "" {
		return ""
	}
	id := host + "/" + repoPath
	// the rest of a path with characters that are not expected in it ends in the query: two repositories must not
	// get the same value because of it
	if u.RawQuery != "" {
		id += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		id += "#" + u.Fragment
	}
	return id
}

// ignoreRules returns the patterns of the ignore file that BuildKit applies to the build context:
// "<dockerfile>.dockerignore" if it exists, ".dockerignore" in the build context otherwise
func (s *smartBuildContext) ignoreRules(contextPath, dockerfile string) []string {
	candidates := []string{filepath.Join(contextPath, ".dockerignore")}
	if dockerfile != "" {
		candidates = append([]string{dockerfile + ".dockerignore"}, candidates...)
	}
	for _, candidate := range candidates {
		content, err := afero.ReadFile(s.fs, candidate)
		if err != nil {
			continue
		}
		return ignorePatterns(content)
	}
	return nil
}

// ignorePatterns returns the lines of an ignore file that are patterns, as they are written. The result does not
// depend on the operating system of the client, so every client gets the same key for the same file.
func ignorePatterns(content []byte) []string {
	var patterns []string
	scanner := bufio.NewScanner(bytes.NewReader(bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))))
	scanner.Buffer(nil, len(content)+1)
	for scanner.Scan() {
		line := scanner.Text()
		// as BuildKit reads it: a comment has # in the first column, and the spaces around a pattern do not count
		if strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// source identifies the directory that the files are sent from (contextPath, without symbolic links). Modification
// times are only comparable between builds of the same directory. In the installer and in CI, where a job usually is a new clone in the same path,
// and where there is no stable machine ID (a container), the source is new on every build, so BuildKit compares
// the files by content.
func (s *smartBuildContext) source(contextPath string) string {
	if s.inInstaller() || s.inCI() {
		return s.randomID()
	}
	machineID, err := s.machineID()
	if err != nil || !isStableMachineID(machineID) {
		return s.randomID()
	}
	return hashID(machineID, contextPath)
}

// isStableMachineID reports whether a machine ID identifies one machine. Container and cloud images are built with
// an empty or not initialized machine ID, which is then the same in every container that has no init system.
func isStableMachineID(machineID string) bool {
	machineID = strings.TrimSpace(machineID)
	return machineID != "" && machineID != "uninitialized"
}

// hashID returns an identifier that does not reveal the values that it is made of
func hashID(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:buildContextIDLength]
}

func randomSource() string {
	b := make([]byte, randomSourceBytes)
	// crypto/rand.Read never returns an error and always fills b
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
