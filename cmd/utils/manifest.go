// Copyright 2021 The Okteto Authors
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

package utils

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"gopkg.in/yaml.v2"
)

type ManifestExecutor interface {
	Execute(command string, env []string) error
}

type Executor struct{}

// NewExecutor returns a new executor
func NewExecutor() *Executor {
	return &Executor{}
}

// Manifest represents a manifest file
type Manifest struct {
	Name     string `yaml:"name"`
	Icon     string `yaml:"icon,omitempty"`
	Type     string
	Deploy   []string `yaml:"deploy,omitempty"`
	Destroy  []string `yaml:"destroy,omitempty"`
	Devs     []string `yaml:"devs,omitempty"`
	Filename string   `yaml:"-"`
}

// InferApplicationName infers the application name from the folder received as parameter
func InferApplicationName(cwd string) string {
	repo, err := model.GetRepositoryURL(cwd)
	if err != nil {
		log.Info("inferring name from folder")
		return filepath.Base(cwd)
	}

	log.Info("inferring name from git repository URL")
	return model.TranslateURLToName(repo)
}

// GetManifest Loads a manifest
func GetManifest(srcFolder, name, filename string) (*Manifest, error) {
	pipelinePath := getPipelinePath(srcFolder, filename)
	if pipelinePath != "" {
		log.Debugf("Found okteto manifest %s", pipelinePath)
		pipelineBytes, err := ioutil.ReadFile(pipelinePath)
		if err != nil {
			return nil, err
		}
		result := &Manifest{}
		if err := yaml.Unmarshal(pipelineBytes, result); err != nil {
			return nil, err
		}
		result.Type = "pipeline"
		result.Filename = pipelinePath
		return result, nil
	}

	src := srcFolder
	path := filepath.Join(srcFolder, filename)
	if filename != "" && pathExistsAndDir(path) {
		src = path
	}

	oktetoSubPath := getOktetoSubPath(srcFolder, src)
	devs := []string{}
	if oktetoSubPath != "" {
		devs = append(devs, oktetoSubPath)
	}
	chartSubPath := getChartsSubPath(srcFolder, src)
	if chartSubPath != "" {
		fmt.Println("Found chart")
		return &Manifest{
			Type:     "chart",
			Deploy:   []string{fmt.Sprintf("helm upgrade --install %s %s", name, chartSubPath)},
			Devs:     devs,
			Filename: chartSubPath,
		}, nil
	}

	manifestsSubPath := getManifestsSubPath(srcFolder, src)
	if manifestsSubPath != "" {
		fmt.Println("Found kubernetes manifests")
		return &Manifest{
			Type:     "kubernetes",
			Deploy:   []string{fmt.Sprintf("kubectl apply -f %s", manifestsSubPath)},
			Devs:     devs,
			Filename: manifestsSubPath,
		}, nil
	}

	stackSubPath := getStackSubPath(srcFolder, src)
	if stackSubPath != "" {
		fmt.Println("Found okteto stack")
		return &Manifest{
			Type:     "stack",
			Deploy:   []string{fmt.Sprintf("okteto stack deploy --build -f %s", stackSubPath)},
			Devs:     devs,
			Filename: stackSubPath,
		}, nil
	}

	if oktetoSubPath != "" {
		fmt.Println("Found okteto manifest")
		return &Manifest{
			Type:     "okteto",
			Deploy:   []string{"okteto push --deploy"},
			Devs:     devs,
			Filename: oktetoSubPath,
		}, nil
	}

	return nil, fmt.Errorf("file okteto manifest not found. See https://okteto.com/docs/cloud/okteto-pipeline for details on how to configure your git repository with okteto")
}

func getPipelinePath(src, filename string) string {
	path := filepath.Join(src, filename)
	if filename != "" && fileExistsAndNotDir(path) {
		return path
	}

	baseDir := src
	if filename != "" && pathExistsAndDir(path) {
		baseDir = path
	}

	// Files will be checked in the order defined in the list
	files := []string{
		"okteto-pipeline.yml",
		"okteto-pipeline.yaml",
		"okteto-pipelines.yml",
		"okteto-pipelines.yaml",
		".okteto/okteto-pipeline.yml",
		".okteto/okteto-pipeline.yaml",
		".okteto/okteto-pipelines.yml",
		".okteto/okteto-pipelines.yaml",
	}

	for _, name := range files {
		path := filepath.Join(baseDir, name)
		if fileExistsAndNotDir(path) {
			return path
		}
	}
	return ""
}

func getChartsSubPath(basePath, src string) string {
	// Files will be checked in the order defined in the list
	for _, name := range []string{"chart", "charts", "helm/chart", "helm/charts"} {
		path := filepath.Join(src, name, "Chart.yaml")
		if model.FileExists(path) {
			prefix := fmt.Sprintf("%s/", basePath)
			return strings.TrimPrefix(path, prefix)
		}
	}
	return ""
}

func getManifestsSubPath(basePath, src string) string {
	// Files will be checked in the order defined in the list
	for _, name := range []string{"manifests", "manifests.yml", "manifests.yaml", "kubernetes", "kubernetes.yml", "kubernetes.yaml", "k8s", "k8s.yml", "k8s.yaml"} {
		path := filepath.Join(src, name)
		if model.FileExists(path) {
			prefix := fmt.Sprintf("%s/", basePath)
			return strings.TrimPrefix(path, prefix)
		}
	}
	return ""
}

func getStackSubPath(basePath, src string) string {
	// Files will be checked in the order defined in the list
	files := []string{
		"okteto-stack.yml",
		"okteto-stack.yaml",
		"stack.yml",
		"stack.yaml",
		".okteto/okteto-stack.yml",
		".okteto/okteto-stack.yaml",
		".okteto/stack.yml",
		".okteto/stack.yaml",
		"docker-compose.yml",
		"docker-compose.yaml",
		".okteto/docker-compose.yml",
		".okteto/docker-compose.yaml",
	}
	for _, name := range files {
		path := filepath.Join(src, name)
		if fileExistsAndNotDir(path) {
			prefix := fmt.Sprintf("%s/", basePath)
			return strings.TrimPrefix(path, prefix)
		}
	}
	return ""
}

func getOktetoSubPath(basePath, src string) string {
	// Files will be checked in the order defined in the list
	files := []string{
		"okteto.yml",
		"okteto.yaml",
		".okteto/okteto.yml",
		".okteto/okteto.yaml",
	}
	for _, name := range files {
		path := filepath.Join(src, name)
		if fileExistsAndNotDir(path) {
			prefix := fmt.Sprintf("%s/", basePath)
			return strings.TrimPrefix(path, prefix)
		}
	}
	return ""
}

func pathExistsAndDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func fileExistsAndNotDir(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Execute executes the specified command adding `env` to the execution environment
func (*Executor) Execute(command string, env []string) error {
	log.Information("Running '%s'...", command)

	cmd := exec.Command("bash", "-c", command)
	cmd.Env = append(os.Environ(), env...)

	r, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	cmd.Stderr = cmd.Stdout
	done := make(chan struct{})
	scanner := bufio.NewScanner(r)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
		}
		done <- struct{}{}
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}
