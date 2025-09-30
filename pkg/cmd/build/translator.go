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
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/registry"
)

const (
	tmpFilePrefix = "buildkit-"
)

type opener interface {
	Open(file string) (io.ReadWriter, error)
}

type fileOpener struct{}

func (fileOpener) Open(file string) (io.ReadWriter, error) {
	return os.Open(file)
}

type NameWriter interface {
	Name() string
	io.Writer
}

type tmpFileCreator interface {
	Create(dir string) (string, error)
}
type osTmpFileCreator struct{}

func (osTmpFileCreator) Create(dir string) (string, error) {
	file, err := os.CreateTemp(dir, tmpFilePrefix)
	if err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return file.Name(), nil
}

type DockerfileTranslator struct {
	opener         opener
	tmpFileCreator tmpFileCreator
	tmpFolder      string
	tmpFileName    string
	translators    []translator
}

func newDockerfileTranslator(okCtx OktetoContextInterface, repoURL, dockerfilePath, target string) (*DockerfileTranslator, error) {

	dockerfileTmpFolder := filepath.Join(config.GetOktetoHome(), ".dockerfile")
	if err := os.MkdirAll(dockerfileTmpFolder, 0700); err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", dockerfileTmpFolder, err)
	}

	return &DockerfileTranslator{
		opener:         fileOpener{},
		tmpFileCreator: osTmpFileCreator{},
		tmpFolder:      dockerfileTmpFolder,
		translators: []translator{
			newRegistryTranslator(okCtx),
			newCacheMountTranslator(repoURL, dockerfilePath, target),
		},
	}, nil
}

func (dt *DockerfileTranslator) translate(filename string) error {
	readerFile, err := dt.opener.Open(filename)
	if err != nil {
		return err
	}

	dt.tmpFileName, err = dt.tmpFileCreator.Create(dt.tmpFolder)
	if err != nil {
		return err
	}

	writerFile, err := dt.opener.Open(dt.tmpFileName)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(readerFile)
	datawriter := bufio.NewWriter(writerFile)
	defer datawriter.Flush()

	for scanner.Scan() {
		line := scanner.Text()

		result := line
		for _, translator := range dt.translators {
			result = translator.translate(result)
		}

		_, err := datawriter.WriteString(result + "\n")
		if err != nil {
			return fmt.Errorf("failed to write dockerfile: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

type translator interface {
	translate(line string) string
}

type registryTranslator struct {
	replacer        registry.Replacer
	userNs          string
	globalNamespace string
}

func newRegistryTranslator(okCtx OktetoContextInterface) registryTranslator {
	globalNamespace := constants.DefaultGlobalNamespace
	ctxGlobalNamespace := okCtx.GetGlobalNamespace()
	if ctxGlobalNamespace != "" {
		globalNamespace = ctxGlobalNamespace
	}

	return registryTranslator{
		replacer:        registry.NewRegistryReplacer(GetRegistryConfigFromOktetoConfig(okCtx)),
		userNs:          okCtx.GetNamespace(),
		globalNamespace: globalNamespace,
	}
}

func (rt registryTranslator) translate(line string) string {
	if strings.Contains(line, constants.DevRegistry) {
		result := rt.replacer.Replace(line, constants.DevRegistry, rt.userNs)
		return result
	}

	if strings.Contains(line, constants.GlobalRegistry) {
		result := rt.replacer.Replace(line, constants.GlobalRegistry, rt.globalNamespace)
		return result
	}
	return line
}

type cacheMountTranslator struct {
	projectHash string
}

func newCacheMountTranslator(repo, dockerfilePath, target string) cacheMountTranslator {
	return cacheMountTranslator{
		projectHash: generateProjectHash(repo, dockerfilePath, target),
	}
}

func generateProjectHash(repositoryURL, manifestName, path string) string {
	// Create input string for hashing
	input := fmt.Sprintf("%s-%s-%s", repositoryURL, manifestName, path)

	// Generate SHA256 hash
	hasher := sha256.New()
	hasher.Write([]byte(input))
	hash := hasher.Sum(nil)

	// Return first 12 characters of hex hash for readability
	return hex.EncodeToString(hash)[:12]
}

func (cmt cacheMountTranslator) translate(line string) string {
	// Check if this RUN command has a cache mount
	hasCacheMount, err := regexp.MatchString(`^RUN.*--mount=.*type=cache`, line)
	if err != nil || !hasCacheMount {
		return line
	}

	// If an id is already defined, leave unchanged
	hasID, err := regexp.MatchString(`^RUN.*--mount=[^ ]*id=`, line)
	if err == nil && hasID {
		return line
	}

	target := ""
	re := regexp.MustCompile(`--mount=[^ ]*target=([^, ]+)`)
	if matches := re.FindStringSubmatch(line); len(matches) > 1 {
		target = matches[1]
	}

	id := cmt.projectHash
	if target != "" {
		id = fmt.Sprintf("%s-%s", cmt.projectHash, target)
	}
	// Otherwise, insert id=<projectHash> into the mount
	return strings.ReplaceAll(line, "--mount=", fmt.Sprintf("--mount=id=%s,", id))
}
