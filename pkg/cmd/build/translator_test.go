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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Mock implementations for testing
type mockOpener struct {
	files      map[string]*mockFile
	shouldFail bool
}

type mockFile struct {
	content string
	pos     int
}

func (mf *mockFile) Read(p []byte) (n int, err error) {
	if mf.pos >= len(mf.content) {
		return 0, io.EOF
	}
	n = copy(p, mf.content[mf.pos:])
	mf.pos += n
	return n, nil
}

func (mf *mockFile) Write(p []byte) (n int, err error) {
	mf.content += string(p)
	return len(p), nil
}

func (mf *mockFile) Close() error {
	return nil
}

func newMockFile(content string) *mockFile {
	return &mockFile{content: content, pos: 0}
}

func (mo *mockOpener) Open(file string) (io.ReadWriteCloser, error) {
	if mo.shouldFail {
		return nil, fmt.Errorf("mock open error")
	}
	if f, exists := mo.files[file]; exists {
		return f, nil
	}
	// Create a new empty file if it doesn't exist
	mo.files[file] = newMockFile("")
	return mo.files[file], nil
}

type mockTmpFileCreator struct {
	shouldFail bool
	fileName   string
}

func (mtfc *mockTmpFileCreator) Create(dir string) (string, error) {
	if mtfc.shouldFail {
		return "", fmt.Errorf("mock tmp file creation error")
	}
	return mtfc.fileName, nil
}

type mockOktetoContext struct {
	namespace       string
	globalNamespace string
	registryURL     string
}

func (moc *mockOktetoContext) GetCurrentName() string                            { return "test" }
func (moc *mockOktetoContext) GetNamespace() string                              { return moc.namespace }
func (moc *mockOktetoContext) GetGlobalNamespace() string                        { return moc.globalNamespace }
func (moc *mockOktetoContext) GetCurrentBuilder() string                         { return "test" }
func (moc *mockOktetoContext) GetCurrentCertStr() string                         { return "" }
func (moc *mockOktetoContext) GetCurrentCfg() *clientcmdapi.Config               { return nil }
func (moc *mockOktetoContext) GetCurrentToken() string                           { return "" }
func (moc *mockOktetoContext) GetCurrentUser() string                            { return "" }
func (moc *mockOktetoContext) ExistsContext() bool                               { return true }
func (moc *mockOktetoContext) IsOktetoCluster() bool                             { return true }
func (moc *mockOktetoContext) IsInsecure() bool                                  { return false }
func (moc *mockOktetoContext) UseContextByBuilder()                              {}
func (moc *mockOktetoContext) GetTokenByContextName(name string) (string, error) { return "", nil }
func (moc *mockOktetoContext) GetRegistryURL() string                            { return moc.registryURL }

type mockReplacerConfig struct {
	registryURL string
}

func (mrc *mockReplacerConfig) GetRegistryURL() string {
	return mrc.registryURL
}

func TestNewDockerfileTranslator(t *testing.T) {
	tests := []struct {
		name                    string
		okCtx                   OktetoContextInterface
		repoURL                 string
		dockerfilePath          string
		target                  string
		expectedTranslatorCount int
	}{
		{
			name: "successful creation with default global namespace",
			okCtx: &mockOktetoContext{
				namespace:       "test-ns",
				globalNamespace: "",
				registryURL:     "registry.example.com",
			},
			repoURL:                 "https://github.com/test/repo",
			dockerfilePath:          "Dockerfile",
			target:                  "prod",
			expectedTranslatorCount: 2,
		},
		{
			name: "successful creation with custom global namespace",
			okCtx: &mockOktetoContext{
				namespace:       "user-ns",
				globalNamespace: "custom-global",
				registryURL:     "registry.example.com",
			},
			repoURL:                 "https://github.com/test/repo",
			dockerfilePath:          "Dockerfile",
			target:                  "dev",
			expectedTranslatorCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dt, err := newDockerfileTranslator(tt.okCtx, tt.repoURL, tt.dockerfilePath, tt.target)

			require.NoError(t, err)
			require.NotNil(t, dt)
			require.Len(t, dt.translators, tt.expectedTranslatorCount)
			require.NotEmpty(t, dt.tmpFolder)
		})
	}
}

func TestDockerfileTranslator_Translate(t *testing.T) {
	tests := []struct {
		name            string
		expectedOutput  string
		setupTranslator func() *DockerfileTranslator
	}{
		{
			name:           "simple dockerfile with no translations needed",
			expectedOutput: "FROM ubuntu:20.04\nRUN echo hello\n",
			setupTranslator: func() *DockerfileTranslator {
				mockOpener := &mockOpener{
					files: make(map[string]*mockFile),
				}
				mockOpener.files["input.dockerfile"] = newMockFile("FROM ubuntu:20.04\nRUN echo hello")

				return &DockerfileTranslator{
					opener:         mockOpener,
					tmpFileCreator: &mockTmpFileCreator{fileName: "temp.dockerfile"},
					tmpFolder:      "/tmp",
					translators:    []translator{}, // No translators
				}
			},
		},
		{
			name:           "dockerfile with dev registry translation",
			expectedOutput: "FROM registry.example.com/test-ns/myimage:latest\n",
			setupTranslator: func() *DockerfileTranslator {
				mockOpener := &mockOpener{
					files: make(map[string]*mockFile),
				}
				mockOpener.files["input.dockerfile"] = newMockFile(fmt.Sprintf("FROM %s/myimage:latest", constants.DevRegistry))

				registryTranslator := registryTranslator{
					replacer:        registry.NewRegistryReplacer(&mockReplacerConfig{registryURL: "registry.example.com"}),
					userNs:          "test-ns",
					globalNamespace: "okteto",
				}

				return &DockerfileTranslator{
					opener:         mockOpener,
					tmpFileCreator: &mockTmpFileCreator{fileName: "temp.dockerfile"},
					tmpFolder:      "/tmp",
					translators:    []translator{registryTranslator},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dt := tt.setupTranslator()

			err := dt.translate("input.dockerfile")
			require.NoError(t, err)

			// Check the output content
			outputFile := dt.opener.(*mockOpener).files[dt.tmpFileName]
			require.NotNil(t, outputFile)
			assert.Equal(t, tt.expectedOutput, outputFile.content)
		})
	}
}

type fakeTranslator struct {
	called bool
}

func (ft *fakeTranslator) translate(line string) string {
	ft.called = true
	return line
}

func TestDockerfileTranslator_AllTranslatorsUsed(t *testing.T) {
	t1 := &fakeTranslator{}
	t2 := &fakeTranslator{}
	t3 := &fakeTranslator{}

	mockOpener := &mockOpener{
		files: make(map[string]*mockFile),
	}
	mockOpener.files["input.dockerfile"] = newMockFile(fmt.Sprintf("FROM %s/myimage:latest", constants.DevRegistry))
	dt := &DockerfileTranslator{
		opener:         mockOpener,
		tmpFileCreator: &mockTmpFileCreator{fileName: "temp.dockerfile"},
		tmpFolder:      "/tmp",
		translators:    []translator{t1, t2, t3},
	}

	err := dt.translate("input.dockerfile")
	require.NoError(t, err)

	assert.True(t, t1.called)
	assert.True(t, t2.called)
	assert.True(t, t3.called)
}

func TestDockerfileTranslator_Translate_Failures(t *testing.T) {
	tests := []struct {
		name            string
		setupTranslator func() *DockerfileTranslator
		filename        string
		expectedError   string
	}{
		{
			name: "opener fails to open input file",
			setupTranslator: func() *DockerfileTranslator {
				return &DockerfileTranslator{
					opener:         &mockOpener{shouldFail: true},
					tmpFileCreator: &mockTmpFileCreator{fileName: "temp.dockerfile"},
					tmpFolder:      "/tmp",
					translators:    []translator{},
				}
			},
			filename:      "nonexistent.dockerfile",
			expectedError: "mock open error",
		},
		{
			name: "tmp file creator fails",
			setupTranslator: func() *DockerfileTranslator {
				mockOpener := &mockOpener{
					files: make(map[string]*mockFile),
				}
				mockOpener.files["input.dockerfile"] = newMockFile("FROM ubuntu:20.04")

				return &DockerfileTranslator{
					opener:         mockOpener,
					tmpFileCreator: &mockTmpFileCreator{shouldFail: true},
					tmpFolder:      "/tmp",
					translators:    []translator{},
				}
			},
			filename:      "input.dockerfile",
			expectedError: "mock tmp file creation error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dt := tt.setupTranslator()

			err := dt.translate(tt.filename)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestRegistryTranslator_Translate(t *testing.T) {
	rt := registryTranslator{
		replacer:        registry.NewRegistryReplacer(&mockReplacerConfig{registryURL: "registry.example.com"}),
		userNs:          "test-ns",
		globalNamespace: "global-ns",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "translates dev registry",
			input:    fmt.Sprintf("FROM %s/myimage:latest", constants.DevRegistry),
			expected: "FROM registry.example.com/test-ns/myimage:latest",
		},
		{
			name:     "translates global registry",
			input:    fmt.Sprintf("FROM %s/shared:v1.0", constants.GlobalRegistry),
			expected: "FROM registry.example.com/global-ns/shared:v1.0",
		},
		{
			name:     "handles dev registry at start of line",
			input:    fmt.Sprintf("%s/builder:latest", constants.DevRegistry),
			expected: "registry.example.com/test-ns/builder:latest",
		},
		{
			name:     "handles dev registry with whitespace prefix",
			input:    fmt.Sprintf("COPY --from %s/builder:latest /app .", constants.DevRegistry),
			expected: "COPY --from registry.example.com/test-ns/builder:latest /app .",
		},
		{
			name:     "leaves non-okteto registries unchanged",
			input:    "FROM docker.io/ubuntu:20.04",
			expected: "FROM docker.io/ubuntu:20.04",
		},
		{
			name:     "leaves standard docker hub image unchanged",
			input:    "FROM ubuntu:20.04",
			expected: "FROM ubuntu:20.04",
		},
		{
			name:     "leaves private registry image unchanged",
			input:    "FROM private.registry.com/myapp:latest",
			expected: "FROM private.registry.com/myapp:latest",
		},
		{
			name:     "leaves empty line unchanged",
			input:    "",
			expected: "",
		},
		{
			name:     "leaves comment line unchanged",
			input:    "# This is a comment",
			expected: "# This is a comment",
		},
		{
			name:     "leaves run command without registry unchanged",
			input:    "RUN apt-get update",
			expected: "RUN apt-get update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rt.translate(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateProjectHash(t *testing.T) {
	tests := []struct {
		name               string
		repositoryURL      string
		manifestName       string
		path               string
		target             string
		expectedHashLength int
		comparisonInputs   []string
		shouldBeDifferent  bool
	}{
		{
			name:               "generates consistent hash for same inputs",
			repositoryURL:      "https://github.com/test/repo",
			manifestName:       "Dockerfile",
			path:               "production",
			target:             "production",
			expectedHashLength: 12,
			comparisonInputs:   []string{"https://github.com/test/repo", "Dockerfile", "production", "production"},
			shouldBeDifferent:  false,
		},
		{
			name:               "different repos produce different hashes",
			repositoryURL:      "https://github.com/different/repo",
			manifestName:       "Dockerfile",
			path:               "production",
			target:             "production",
			expectedHashLength: 12,
			comparisonInputs:   []string{"https://github.com/test/repo", "Dockerfile", "production", "production"},
			shouldBeDifferent:  true,
		},
		{
			name:               "different paths produce different hashes",
			repositoryURL:      "https://github.com/test/repo",
			manifestName:       "Dockerfile",
			path:               "development",
			target:             "development",
			expectedHashLength: 12,
			comparisonInputs:   []string{"https://github.com/test/repo", "Dockerfile", "production", "production"},
			shouldBeDifferent:  true,
		},
		{
			name:               "handles empty values",
			repositoryURL:      "",
			manifestName:       "",
			path:               "",
			target:             "",
			expectedHashLength: 12,
			comparisonInputs:   []string{"https://github.com/test/repo", "Dockerfile", "production", "production"},
			shouldBeDifferent:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := generateProjectHash(tt.repositoryURL, tt.manifestName, tt.path, tt.target)

			assert.Len(t, hash, tt.expectedHashLength)
			assert.NotEmpty(t, hash)

			// Test consistency
			hash2 := generateProjectHash(tt.repositoryURL, tt.manifestName, tt.path, tt.target)
			assert.Equal(t, hash, hash2)

			// Test comparison
			comparisonHash := generateProjectHash(tt.comparisonInputs[0], tt.comparisonInputs[1], tt.comparisonInputs[2], tt.comparisonInputs[3])
			assert.Equal(t, tt.shouldBeDifferent, hash != comparisonHash)
		})
	}
}

func TestCacheMountTranslator_Translate(t *testing.T) {
	cmt := newCacheMountTranslator("https://github.com/test/repo", "Dockerfile", "prod")
	// Override projectHash for predictable test results
	cmt.hash = func(repositoryURL, manifestName, path, target string) string {
		return fmt.Sprintf("%s-%s-%s-%s", repositoryURL, manifestName, path, target)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "adds id to cache mount without target",
			input:    "RUN --mount=type=cache apt-get update",
			expected: "RUN --mount=id=https://github.com/test/repo-Dockerfile-prod-,type=cache apt-get update",
		},
		{
			name:     "adds id with target to cache mount",
			input:    "RUN --mount=type=cache,target=/root/.cache pip install -r requirements.txt",
			expected: "RUN --mount=id=https://github.com/test/repo-Dockerfile-prod-/root/.cache,type=cache,target=/root/.cache pip install -r requirements.txt",
		},
		{
			name:     "handles complex cache mount with multiple parameters",
			input:    "RUN --mount=type=cache,target=/var/cache/apt,sharing=locked apt-get update",
			expected: "RUN --mount=id=https://github.com/test/repo-Dockerfile-prod-/var/cache/apt,type=cache,target=/var/cache/apt,sharing=locked apt-get update",
		},
		{
			name:     "handles cache mount with quoted target",
			input:    `RUN --mount=type=cache,target="/go/pkg/mod" go mod download`,
			expected: `RUN --mount=id=https://github.com/test/repo-Dockerfile-prod-"/go/pkg/mod",type=cache,target="/go/pkg/mod" go mod download`,
		},
		{
			name:     "leaves non-RUN command unchanged",
			input:    "FROM ubuntu:20.04",
			expected: "FROM ubuntu:20.04",
		},
		{
			name:     "leaves RUN without cache mount unchanged",
			input:    "RUN apt-get update",
			expected: "RUN apt-get update",
		},
		{
			name:     "leaves cache mount with existing id unchanged",
			input:    "RUN --mount=type=cache,id=custom-id,target=/cache apt-get update",
			expected: "RUN --mount=type=cache,id=custom-id,target=/cache apt-get update",
		},
		{
			name:     "leaves RUN with different mount type unchanged",
			input:    "RUN --mount=type=bind,source=.,target=/app ls",
			expected: "RUN --mount=type=bind,source=.,target=/app ls",
		},
		{
			name:     "leaves empty line unchanged",
			input:    "",
			expected: "",
		},
		{
			name:     "leaves comment line unchanged",
			input:    "# Install dependencies",
			expected: "# Install dependencies",
		},
		{
			name:     "leaves RUN with secret mount unchanged",
			input:    "RUN --mount=type=secret,id=mysecret cat /run/secrets/mysecret",
			expected: "RUN --mount=type=secret,id=mysecret cat /run/secrets/mysecret",
		},
		{
			name:     "handles multiple cache mounts in single RUN command",
			input:    "RUN --mount=type=cache,target=./.eslintcache --mount=type=cache,target=./.yarn/cache,sharing=private --mount=type=cache,target=./node_modules,sharing=private yarn install --immutable",
			expected: "RUN --mount=id=https://github.com/test/repo-Dockerfile-prod-./.eslintcache,type=cache,target=./.eslintcache --mount=id=https://github.com/test/repo-Dockerfile-prod-./.yarn/cache,type=cache,target=./.yarn/cache,sharing=private --mount=id=https://github.com/test/repo-Dockerfile-prod-./node_modules,type=cache,target=./node_modules,sharing=private yarn install --immutable",
		},
		{
			name:     "handles multiple cache mounts in single RUN command, some with id",
			input:    "RUN --mount=id=test,type=cache,target=./.eslintcache --mount=type=cache,target=./.yarn/cache,sharing=private --mount=type=cache,target=./node_modules,sharing=private yarn install --immutable",
			expected: "RUN --mount=id=test,type=cache,target=./.eslintcache --mount=id=https://github.com/test/repo-Dockerfile-prod-./.yarn/cache,type=cache,target=./.yarn/cache,sharing=private --mount=id=https://github.com/test/repo-Dockerfile-prod-./node_modules,type=cache,target=./node_modules,sharing=private yarn install --immutable",
		},
		{
			name:     "handles one cache mount without id but with uid and gid",
			input:    "RUN --mount=type=cache,uid=1000,gid=1000,target=./.eslintcache --mount=type=cache,target=./.yarn/cache,sharing=private --mount=type=cache,target=./node_modules,sharing=private yarn install --immutable",
			expected: "RUN --mount=id=https://github.com/test/repo-Dockerfile-prod-./.eslintcache,type=cache,uid=1000,gid=1000,target=./.eslintcache --mount=id=https://github.com/test/repo-Dockerfile-prod-./.yarn/cache,type=cache,target=./.yarn/cache,sharing=private --mount=id=https://github.com/test/repo-Dockerfile-prod-./node_modules,type=cache,target=./node_modules,sharing=private yarn install --immutable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmt.translate(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOsTmpFileCreator_Create(t *testing.T) {
	tests := []struct {
		name        string
		setupTmpDir func(*testing.T) string
	}{
		{
			name: "creates temp file with correct prefix",
			setupTmpDir: func(t *testing.T) string {
				return t.TempDir()
			},
		},
		{
			name: "creates temp file in nested directory",
			setupTmpDir: func(t *testing.T) string {
				baseDir := t.TempDir()
				nestedDir := filepath.Join(baseDir, "nested")
				require.NoError(t, os.MkdirAll(nestedDir, 0755))
				return nestedDir
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creator := osTmpFileCreator{}
			tmpDir := tt.setupTmpDir(t)

			filename, err := creator.Create(tmpDir)
			require.NoError(t, err)

			// Verify file was created with correct prefix and location
			assert.True(t, strings.HasPrefix(filepath.Base(filename), tmpFilePrefix))
			assert.True(t, strings.HasPrefix(filename, tmpDir))

			// Verify file exists and can be written to
			file, err := os.OpenFile(filename, os.O_RDWR, 0644)
			require.NoError(t, err)
			defer file.Close()

			_, err = file.WriteString("test content")
			require.NoError(t, err)
		})
	}
}

func TestFileOpener_Open(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		setupFile func(*testing.T, string) string
	}{
		{
			name:    "opens existing file with content",
			content: "test content",
			setupFile: func(t *testing.T, content string) string {
				tmpFile, err := os.CreateTemp(t.TempDir(), "test-")
				require.NoError(t, err)

				_, err = tmpFile.WriteString(content)
				require.NoError(t, err)
				require.NoError(t, tmpFile.Close())

				return tmpFile.Name()
			},
		},
		{
			name:    "opens empty file",
			content: "",
			setupFile: func(t *testing.T, content string) string {
				tmpFile, err := os.CreateTemp(t.TempDir(), "empty-")
				require.NoError(t, err)
				require.NoError(t, tmpFile.Close())

				return tmpFile.Name()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opener := fileOpener{}
			filename := tt.setupFile(t, tt.content)
			defer os.Remove(filename)

			file, err := opener.Open(filename)
			require.NoError(t, err)
			defer file.Close()

			// Verify we can read from it
			content, err := io.ReadAll(file)
			require.NoError(t, err)
			assert.Equal(t, tt.content, string(content))

			// Verify we can close it
			err = file.Close()
			require.NoError(t, err)
		})
	}
}

func TestFileOpener_Open_Failures(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "nonexistent file returns error",
			filePath: "/nonexistent/file/path",
		},
		{
			name:     "invalid file path returns error",
			filePath: "/invalid\x00path/file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opener := fileOpener{}

			_, err := opener.Open(tt.filePath)
			require.Error(t, err)
		})
	}
}

func TestDockerfileTranslator_Integration(t *testing.T) {
	tests := []struct {
		name                 string
		dockerfileContent    string
		okCtx                OktetoContextInterface
		repoURL              string
		target               string
		expectedContains     []string
		expectedCacheMountID bool
	}{
		{
			name: "simple dockerfile translation",
			dockerfileContent: `FROM ubuntu:20.04
RUN --mount=type=cache,target=/var/cache/apt apt-get update
FROM alpine:latest`,
			okCtx: &mockOktetoContext{
				namespace:       "user-ns",
				globalNamespace: "global-ns",
				registryURL:     "registry.example.com",
			},
			repoURL: "https://github.com/test/repo",
			target:  "prod",
			expectedContains: []string{
				"FROM ubuntu:20.04",
				"FROM alpine:latest",
			},
			expectedCacheMountID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			dockerfilePath := filepath.Join(tmpDir, "Dockerfile")

			err := os.WriteFile(dockerfilePath, []byte(tt.dockerfileContent), 0644)
			require.NoError(t, err)

			inputContent, err := os.ReadFile(dockerfilePath)
			require.NoError(t, err)
			require.NotEmpty(t, string(inputContent))
			t.Logf("Input Dockerfile content: %q", string(inputContent))

			dt, err := newDockerfileTranslator(tt.okCtx, tt.repoURL, dockerfilePath, tt.target)
			require.NoError(t, err)

			err = dt.translate(dockerfilePath)
			require.NoError(t, err)

			require.NotEmpty(t, dt.tmpFileName, "temporary file name should be set after translation")
			t.Logf("Temporary file created: %s", dt.tmpFileName)

			_, err = os.Stat(dt.tmpFileName)
			require.NoError(t, err, "temporary file should exist")

			translatedContent, err := os.ReadFile(dt.tmpFileName)
			require.NoError(t, err)
			defer os.Remove(dt.tmpFileName)

			translatedStr := string(translatedContent)
			t.Logf("Translated content: %q", translatedStr)

			assert.NotEmpty(t, translatedStr, "translated content should not be empty")

			for _, expected := range tt.expectedContains {
				assert.Contains(t, translatedStr, expected, "should contain: %s", expected)
			}

			assert.Equal(t, tt.expectedCacheMountID, strings.Contains(translatedStr, "--mount=id="), "cache mount ID should be present")
		})
	}
}
