// Copyright 2022 The Okteto Authors
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

package model

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/a8m/envsubst"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	yaml "gopkg.in/yaml.v2"
)

//Archetype represents the type of manifest
type Archetype string

var (
	//StackType represents a stack manifest type
	StackType Archetype = "stack"
	//OktetoType represents a okteto manifest type
	OktetoType Archetype = "okteto"
	//OktetoType represents a okteto manifest type
	OktetoManifestType Archetype = "manifest"
	//PipelineType represents a okteto pipeline manifest type
	PipelineType Archetype = "pipeline"
	//KubernetesType represents a k8s manifest type
	KubernetesType Archetype = "kubernetes"
	//ChartType represents a k8s manifest type
	ChartType Archetype = "chart"
)

var (
	pipelineFiles = []string{
		"okteto-pipeline.yml",
		"okteto-pipeline.yaml",
		"okteto-pipelines.yml",
		"okteto-pipelines.yaml",
		".okteto/okteto-pipeline.yml",
		".okteto/okteto-pipeline.yaml",
		".okteto/okteto-pipelines.yml",
		".okteto/okteto-pipelines.yaml",
	}
	stackFiles = []string{
		"okteto-stack.yml",
		"okteto-stack.yaml",
		"stack.yml",
		"stack.yaml",
		".okteto/okteto-stack.yml",
		".okteto/okteto-stack.yaml",
		".okteto/stack.yml",
		".okteto/stack.yaml",

		"okteto-compose.yml",
		"okteto-compose.yaml",
		".okteto/okteto-compose.yml",
		".okteto/okteto-compose.yaml",

		"docker-compose.yml",
		"docker-compose.yaml",
		".okteto/docker-compose.yml",
		".okteto/docker-compose.yaml",
	}
	oktetoFiles = []string{
		"okteto.yml",
		"okteto.yaml",
		".okteto/okteto.yml",
		".okteto/okteto.yaml",
	}
	chartsSubPath = []string{
		"chart",
		"charts",
		"helm/chart",
		"helm/charts",
	}
	manifestSubPath = []string{
		"manifests",
		"manifests.yml",
		"manifests.yaml",
		"kubernetes",
		"kubernetes.yml",
		"kubernetes.yaml",
		"k8s",
		"k8s.yml",
		"k8s.yaml",
	}
)

//Manifest represents an okteto manifest
type Manifest struct {
	Name         string               `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace    string               `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Context      string               `json:"context,omitempty" yaml:"context,omitempty"`
	Icon         string               `json:"icon,omitempty" yaml:"icon,omitempty"`
	Deploy       *DeployInfo          `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	Dev          ManifestDevs         `json:"dev,omitempty" yaml:"dev,omitempty"`
	Destroy      []DeployCommand      `json:"destroy,omitempty" yaml:"destroy,omitempty"`
	Build        ManifestBuild        `json:"build,omitempty" yaml:"build,omitempty"`
	Dependencies ManifestDependencies `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`

	Type     Archetype `json:"-" yaml:"-"`
	Filename string    `yaml:"-"`
	Manifest []byte    `json:"-" yaml:"-"`
	IsV2     bool      `json:"-" yaml:"-"`
}

//ManifestDevs defines all the dev section
type ManifestDevs map[string]*Dev

//ManifestBuild defines all the build section
type ManifestBuild map[string]*BuildInfo

// ManifestDependencies represents the map of dependencies at a manifest
type ManifestDependencies map[string]*Dependency

//NewManifest creates a new empty manifest
func NewManifest() *Manifest {
	return &Manifest{
		Dev:    map[string]*Dev{},
		Build:  map[string]*BuildInfo{},
		Deploy: &DeployInfo{},
	}
}

//NewManifestFromDev creates a manifest from a dev
func NewManifestFromDev(dev *Dev) *Manifest {
	manifest := NewManifest()
	name, err := ExpandEnv(dev.Name)
	if err != nil {
		oktetoLog.Infof("could not expand dev name '%s'", dev.Name)
		name = dev.Name
	}
	manifest.Dev[name] = dev
	return manifest
}

//DeployInfo represents what must be deployed for the app to work
type DeployInfo struct {
	Commands []DeployCommand `json:"commands,omitempty" yaml:"commands,omitempty"`
}

//DeployCommand represents a command to be executed
type DeployCommand struct {
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Command string `json:"command,omitempty" yaml:"command,omitempty"`
}

//NewDeployInfo creates a deploy Info
func NewDeployInfo() *DeployInfo {
	return &DeployInfo{
		Commands: []DeployCommand{},
	}
}

//GetManifestV2 gets a manifest from a path or search for the files to generate it
func GetManifestV2(manifestPath string) (*Manifest, error) {
	if manifestPath != "" && fileExistsAndNotDir(manifestPath) {
		return getManifest(manifestPath)
	} else if manifestPath != "" && pathExistsAndDir(manifestPath) {
		return nil, fmt.Errorf("can not parse a dir path")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	var devManifest *Manifest
	if oktetoPath := getFilePath(cwd, oktetoFiles); oktetoPath != "" {
		oktetoLog.Infof("Found okteto file")
		devManifest, err = GetManifestV2(oktetoPath)
		if err != nil {
			return nil, err
		}
		if devManifest.IsV2 {
			devManifest.Type = OktetoManifestType
			return devManifest, nil
		}
	}

	if pipelinePath := getFilePath(cwd, pipelineFiles); pipelinePath != "" {
		oktetoLog.Infof("Found pipeline")
		pipelineManifest, err := GetManifestV2(pipelinePath)
		if err != nil {
			return nil, err
		}
		pipelineManifest.Type = PipelineType
		if devManifest != nil {
			pipelineManifest.mergeWithOktetoManifest(devManifest)
		}

		return pipelineManifest, nil

	}

	if chartPath := getChartPath(cwd); chartPath != "" {
		oktetoLog.Infof("Found chart")
		chartManifest := &Manifest{
			Type: ChartType,
			Deploy: &DeployInfo{
				Commands: []DeployCommand{
					{
						Name:    fmt.Sprintf("helm upgrade --install ${%s} %s", OktetoNameEnvVar, chartPath),
						Command: fmt.Sprintf("helm upgrade --install ${%s} %s", OktetoNameEnvVar, chartPath),
					},
				},
			},
			Filename: chartPath,
		}
		if devManifest != nil {
			chartManifest.mergeWithOktetoManifest(devManifest)
		}
		return chartManifest, nil

	}
	if manifestPath := getManifestsPath(cwd); manifestPath != "" {
		oktetoLog.Infof("Found kubernetes manifests")
		k8sManifest := &Manifest{
			Type: KubernetesType,
			Deploy: &DeployInfo{
				Commands: []DeployCommand{
					{
						Name:    fmt.Sprintf("kubectl apply -f %s", manifestPath),
						Command: fmt.Sprintf("kubectl apply -f %s", manifestPath),
					},
				},
			},
			Filename: manifestPath,
		}
		if devManifest != nil {
			k8sManifest.mergeWithOktetoManifest(devManifest)
		}
		return k8sManifest, nil
	}

	if stackPath := getFilePath(cwd, stackFiles); stackPath != "" {
		oktetoLog.Infof("Found okteto stack")
		stackManifest := &Manifest{
			Type: StackType,
			Deploy: &DeployInfo{
				Commands: []DeployCommand{
					{
						Name:    fmt.Sprintf("okteto stack deploy --build -f %s", stackPath),
						Command: fmt.Sprintf("okteto stack deploy --build -f %s", stackPath),
					},
				},
			},
			Filename: stackPath,
		}
		if devManifest != nil {
			stackManifest.mergeWithOktetoManifest(devManifest)
		}
		return stackManifest, nil
	}
	if devManifest != nil {
		devManifest.Type = OktetoType
		devManifest.Deploy = &DeployInfo{
			Commands: []DeployCommand{
				{
					Name:    "okteto push --deploy",
					Command: "okteto push --deploy",
				},
			},
		}
		return devManifest, nil
	}
	return nil, oktetoErrors.ErrManifestNotFound
}

// get returns a Dev object from a given file
func getManifest(devPath string) (*Manifest, error) {
	b, err := os.ReadFile(devPath)
	if err != nil {
		return nil, err
	}

	manifest, err := Read(b)
	if err != nil {
		return nil, err
	}

	for _, dev := range manifest.Dev {

		if err := dev.loadAbsPaths(devPath); err != nil {
			return nil, err
		}

		if err := dev.expandEnvFiles(); err != nil {
			return nil, err
		}

		if err := dev.validate(); err != nil {
			return nil, err
		}

		dev.computeParentSyncFolder()
	}

	manifest.Filename = devPath

	return manifest, nil
}

func getFilePath(cwd string, files []string) string {
	for _, name := range files {
		path := filepath.Join(cwd, name)
		if fileExistsAndNotDir(path) {
			return path
		}
	}
	return ""
}

func getChartPath(cwd string) string {
	// Files will be checked in the order defined in the list
	for _, name := range chartsSubPath {
		path := filepath.Join(cwd, name, "Chart.yaml")
		if FileExists(path) {
			return path
		}
	}
	return ""
}

func getManifestsPath(cwd string) string {
	// Files will be checked in the order defined in the list
	for _, name := range manifestSubPath {
		path := filepath.Join(cwd, name)
		if FileExists(path) {
			return path
		}
	}
	return ""
}

// Read reads an okteto manifests
func Read(bytes []byte) (*Manifest, error) {
	manifest := NewManifest()
	if bytes != nil {
		if err := yaml.UnmarshalStrict(bytes, manifest); err != nil {
			if strings.HasPrefix(err.Error(), "yaml: unmarshal errors:") {
				var sb strings.Builder
				_, _ = sb.WriteString("Invalid manifest:\n")
				l := strings.Split(err.Error(), "\n")
				for i := 1; i < len(l); i++ {
					e := strings.TrimSuffix(l[i], "in type model.Manifest")
					e = strings.TrimSpace(e)
					_, _ = sb.WriteString(fmt.Sprintf("    - %s\n", e))
				}

				_, _ = sb.WriteString(fmt.Sprintf("    See %s for details", "https://okteto.com/docs/reference/manifest/"))
				return nil, errors.New(sb.String())
			}

			msg := strings.Replace(err.Error(), "yaml: unmarshal errors:", "invalid manifest:", 1)
			msg = strings.TrimSuffix(msg, "in type model.Manifest")
			return nil, errors.New(msg)
		}
	}
	for dName, d := range manifest.Dev {
		if d.Name == "" {
			d.Name = dName
		}
		if err := d.expandEnvVars(); err != nil {
			return nil, fmt.Errorf("Error on dev '%s': %s", d.Name, err)
		}
		for _, s := range d.Services {
			if err := s.expandEnvVars(); err != nil {
				return nil, fmt.Errorf("Error on dev '%s': %s", d.Name, err)
			}
			if err := s.validateForExtraFields(); err != nil {
				return nil, fmt.Errorf("Error on dev '%s': %s", d.Name, err)
			}
		}

		if err := d.SetDefaults(); err != nil {
			return nil, fmt.Errorf("Error on dev '%s': %s", d.Name, err)
		}
		if err := d.translateDeprecatedMetadataFields(); err != nil {
			return nil, fmt.Errorf("Error on dev '%s': %s", d.Name, err)
		}
		sort.SliceStable(d.Forward, func(i, j int) bool {
			return d.Forward[i].less(&d.Forward[j])
		})

		sort.SliceStable(d.Reverse, func(i, j int) bool {
			return d.Reverse[i].Local < d.Reverse[j].Local
		})

		if err := d.translateDeprecatedVolumeFields(); err != nil {
			return nil, err
		}
	}

	for _, b := range manifest.Build {
		if b.Name != "" {
			b.Context = b.Name
			b.Name = ""
		}
		b.setBuildDefaults()
	}
	manifest.Manifest = bytes
	return manifest, nil
}

func (m *Manifest) mergeWithOktetoManifest(other *Manifest) {
	if len(m.Dev) == 0 && len(other.Dev) != 0 {
		m.Dev = other.Dev
	}
}

// ExpandEnvVars expands env vars to be set on the manifest
func (m *Manifest) ExpandEnvVars() (*Manifest, error) {
	var err error
	if m.Deploy != nil {
		for idx, cmd := range m.Deploy.Commands {
			cmd.Command, err = ExpandEnv(cmd.Command)
			if err != nil {
				return nil, errors.New("could not parse env vars")
			}
			m.Deploy.Commands[idx] = cmd
		}
	}
	if m.Destroy != nil {
		for idx, cmd := range m.Destroy {
			cmd.Command, err = envsubst.String(cmd.Command)
			if err != nil {
				return nil, errors.New("could not parse env vars")
			}
			m.Destroy[idx] = cmd
		}
	}

	return m, nil
}

// Dependency represents a dependency object at the manifest
type Dependency struct {
	Repository   string      `json:"repository" yaml:"repository"`
	ManifestPath string      `json:"manifest,omitempty" yaml:"manifest,omitempty"`
	Branch       string      `json:"branch,omitempty" yaml:"branch,omitempty"`
	Variables    Environment `json:"variables,omitempty" yaml:"variables,omitempty"`
	Wait         bool        `json:"wait,omitempty" yaml:"wait,omitempty"`
}
