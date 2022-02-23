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
	yaml3 "gopkg.in/yaml.v3"
)

// Archetype represents the type of manifest
type Archetype string

var (
	// StackType represents a stack manifest type
	StackType Archetype = "compose"
	// OktetoType represents a okteto manifest type
	OktetoType Archetype = "okteto"
	// OktetoManifestType represents a okteto manifest type
	OktetoManifestType Archetype = "manifest"
	// PipelineType represents a okteto pipeline manifest type
	PipelineType Archetype = "pipeline"
	// KubernetesType represents a k8s manifest type
	KubernetesType Archetype = "kubernetes"
	// ChartType represents a k8s manifest type
	ChartType Archetype = "chart"
)

const (
	buildHeadComment = "The build section defines how to build the images of your development environment\nMore info: https://www.okteto.com/docs/reference/manifest/#build"
	buildExample     = `build:
  my-service:
    context: .`
	buildSvcEnvVars         = "You can use the following env vars to refer to this image in your deploy commands:\n - OKTETO_BUILD_%s_REGISTRY: image registry\n - OKTETO_BUILD_%s_REPOSITORY: image repo\n - OKTETO_BUILD_%s_IMAGE: image name\n - OKTETO_BUILD_%s_TAG: image tag"
	deployHeadComment       = "The deploy section defines how to deploy your development environment\nMore info: https://www.okteto.com/docs/reference/manifest/#deploy"
	devHeadComment          = "The dev section defines how to activate a development container\nMore info: https://www.okteto.com/docs/reference/manifest/#dev"
	dependenciesHeadComment = "The dependencies section defines other git repositories to be deployed as part of your development environment\nMore info: https://www.okteto.com/docs/reference/manifest/#dependencies"
	dependenciesExample     = `dependencies:
  - https://github.com/okteto/b
  - frontend/okteto.yml`

	// FakeCommand prints into terminal a fake command
	FakeCommand = "echo 'Replace this line with the proper 'helm' or 'kubectl' commands to deploy your development environment'"
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

// Manifest represents an okteto manifest
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

// ManifestDevs defines all the dev section
type ManifestDevs map[string]*Dev

// ManifestBuild defines all the build section
type ManifestBuild map[string]*BuildInfo

// ManifestDependencies represents the map of dependencies at a manifest
type ManifestDependencies map[string]*Dependency

// NewManifest creates a new empty manifest
func NewManifest() *Manifest {
	return &Manifest{
		Dev:          map[string]*Dev{},
		Build:        map[string]*BuildInfo{},
		Dependencies: map[string]*Dependency{},
		Deploy:       &DeployInfo{},
	}
}

// NewManifestFromDev creates a manifest from a dev
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

// DeployInfo represents what must be deployed for the app to work
type DeployInfo struct {
	Commands  []DeployCommand `json:"commands,omitempty" yaml:"commands,omitempty"`
	Compose   *ComposeInfo    `json:"compose,omitempty" yaml:"compose,omitempty"`
	Endpoints EndpointSpec    `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
}

// ComposeInfo represents information about compose file
type ComposeInfo struct {
	Manifest []string `json:"manifest,omitempty" yaml:"manifest,omitempty"`
	Stack    *Stack   `json:"-" yaml:"-"`
}

// DeployCommand represents a command to be executed
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
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if manifestPath != "" && FileExistsAndNotDir(manifestPath) {
		return getManifest(manifestPath)
	} else if manifestPath != "" && pathExistsAndDir(manifestPath) {
		cwd = manifestPath
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
			if devManifest.Deploy != nil && devManifest.Deploy.Compose != nil && len(devManifest.Deploy.Compose.Manifest) > 0 {
				s, err := LoadStack("", devManifest.Deploy.Compose.Manifest)
				if err != nil {
					return nil, err
				}
				devManifest.Deploy.Compose.Stack = s
				s.Endpoints = devManifest.Deploy.Endpoints
				devManifest, err = devManifest.InferFromStack()
				if err != nil {
					return nil, err
				}
			}
			return devManifest, nil
		}
	}

	inferredManifest, err := GetInferredManifest(cwd)
	if err != nil {
		return nil, err
	}
	if inferredManifest != nil {
		if devManifest != nil {
			inferredManifest.mergeWithOktetoManifest(devManifest)
		} else if inferredManifest.Type == StackType {
			inferredManifest, err = inferredManifest.InferFromStack()
			if err != nil {
				return nil, err
			}
			//TODO: uncomment
			// inferredManifest.Deploy.Compose.Stack.Endpoints = devManifest.Deploy.Endpoints
		}
		return inferredManifest, nil
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

// GetInferredManifest infers the manifest from a directory
func GetInferredManifest(cwd string) (*Manifest, error) {
	if pipelinePath := getFilePath(cwd, pipelineFiles); pipelinePath != "" {
		oktetoLog.Infof("Found pipeline")
		pipelineManifest, err := GetManifestV2(pipelinePath)
		if err != nil {
			return nil, err
		}
		pipelineManifest.Type = PipelineType
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
			Dev:      ManifestDevs{},
			Build:    ManifestBuild{},
			Filename: chartPath,
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
			Dev:      ManifestDevs{},
			Build:    ManifestBuild{},
			Filename: manifestPath,
		}
		return k8sManifest, nil
	}

	if stackPath := getFilePath(cwd, stackFiles); stackPath != "" {
		oktetoLog.Infof("Found okteto compose")
		stackManifest := &Manifest{
			Type: StackType,
			Deploy: &DeployInfo{
				Compose: &ComposeInfo{
					Manifest: []string{
						stackPath,
					},
				},
			},
			Dev:      ManifestDevs{},
			Build:    ManifestBuild{},
			Filename: stackPath,
			IsV2:     true,
		}
		s, err := LoadStack("", stackManifest.Deploy.Compose.Manifest)
		if err != nil {
			return nil, err
		}
		stackManifest.Deploy.Compose.Stack = s

		for srv := range stackManifest.Deploy.Compose.Stack.Services {
			s := stackManifest.Deploy.Compose.Stack.Services[srv]
			if s.Build != nil {
				stackManifest.Build[srv] = s.Build
			}
		}
		return stackManifest, nil
	}
	return nil, nil
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
		if FileExistsAndNotDir(path) {
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
	if err := manifest.setDefaults(); err != nil {
		return nil, err
	}
	manifest.Manifest = bytes
	return manifest, nil
}

func (m *Manifest) setDefaults() error {
	for dName, d := range m.Dev {
		if d.Name == "" {
			d.Name = dName
		}
		if err := d.expandEnvVars(); err != nil {
			return fmt.Errorf("Error on dev '%s': %s", d.Name, err)
		}
		for _, s := range d.Services {
			if err := s.expandEnvVars(); err != nil {
				return fmt.Errorf("Error on dev '%s': %s", d.Name, err)
			}
			if err := s.validateForExtraFields(); err != nil {
				return fmt.Errorf("Error on dev '%s': %s", d.Name, err)
			}
		}

		if err := d.SetDefaults(); err != nil {
			return fmt.Errorf("Error on dev '%s': %s", d.Name, err)
		}
		if err := d.translateDeprecatedMetadataFields(); err != nil {
			return fmt.Errorf("Error on dev '%s': %s", d.Name, err)
		}
		sort.SliceStable(d.Forward, func(i, j int) bool {
			return d.Forward[i].less(&d.Forward[j])
		})

		sort.SliceStable(d.Reverse, func(i, j int) bool {
			return d.Reverse[i].Local < d.Reverse[j].Local
		})

		if err := d.translateDeprecatedVolumeFields(); err != nil {
			return err
		}
	}

	for _, b := range m.Build {
		if b.Name != "" {
			b.Context = b.Name
			b.Name = ""
		}
		b.setBuildDefaults()
	}
	return nil
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

// InferFromStack infers data from a stackfile
func (m *Manifest) InferFromStack() (*Manifest, error) {
	for svcName, svcInfo := range m.Deploy.Compose.Stack.Services {
		d := NewDev()
		for _, p := range svcInfo.Ports {
			if p.HostPort != 0 {
				d.Forward = append(d.Forward, Forward{Local: int(p.HostPort), Remote: int(p.ContainerPort)})
			}
		}
		for _, v := range svcInfo.VolumeMounts {
			if pathExistsAndDir(v.LocalPath) {
				d.Sync.Folders = append(d.Sync.Folders, SyncFolder{
					LocalPath:  v.LocalPath,
					RemotePath: v.RemotePath,
				})
			}
		}
		d.Command = svcInfo.Command
		d.EnvFiles = svcInfo.EnvFiles
		d.Environment = svcInfo.Environment
		d.Name = svcName

		m.Dev[svcName] = d

		if svcInfo.Build == nil {
			continue
		}
		buildInfo := svcInfo.Build
		m.Build[svcName] = buildInfo
	}
	m.setDefaults()
	return m, nil
}

// WriteToFile writes a manifest to a file with comments to make it easier to understand
func (m *Manifest) WriteToFile(filePath string) error {
	if m.Deploy != nil {
		if len(m.Deploy.Commands) == 0 && m.Deploy.Compose == nil {
			m.Deploy.Commands = []DeployCommand{
				{
					Name:    FakeCommand,
					Command: FakeCommand,
				},
			}
		}
	}
	if len(m.Dev) == 0 {
		m.Dev = ManifestDevs{
			m.Name: NewDev(),
		}
	}
	for _, d := range m.Dev {
		if d.Image.Name != "" {
			d.Image.Context = ""
			d.Image.Dockerfile = ""
		} else {
			d.Image = nil
		}

		if d.Push.Name != "" {
			d.Push.Context = ""
			d.Push.Dockerfile = ""
		} else {
			d.Push = nil
		}
	}
	m.Context = ""
	m.Namespace = ""
	//Unmarshal with yamlv2 because we have the marshal with yaml v2
	bytes, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	doc := yaml3.Node{}
	if err := yaml3.Unmarshal(bytes, &doc); err != nil {
		return err
	}
	doc = *doc.Content[0]
	currentSection := ""
	for idx, section := range doc.Content {
		switch section.Value {
		case "build":
			currentSection = "build"
			section.HeadComment = buildHeadComment
		case "deploy":
			currentSection = "deploy"
			section.HeadComment = deployHeadComment
		case "dev":
			currentSection = "dev"
			section.HeadComment = devHeadComment
		}
		if idx%2 == 0 {
			continue
		}
		if currentSection == "build" {
			for _, subsection := range section.Content {
				if subsection.Kind != yaml3.ScalarNode {
					continue
				}
				serviceName := strings.ToUpper(subsection.Value)
				subsection.HeadComment = fmt.Sprintf(buildSvcEnvVars, serviceName, serviceName, serviceName, serviceName)
			}
		}
	}

	doc = m.reorderDocFields(doc)

	out, err := yaml3.Marshal(doc)
	if err != nil {
		return err
	}
	out = addEmptyLineBetweenSections(out)
	if err := os.WriteFile(filePath, out, 0600); err != nil {
		oktetoLog.Infof("failed to write stignore file: %s", err)
		return err
	}
	return nil
}

// reorderDocFields orders the manifest to be: name -> build -> deploy -> dependencies -> dev
func (m *Manifest) reorderDocFields(doc yaml3.Node) yaml3.Node {
	contentCopy := []*yaml3.Node{}
	nodes := []int{}
	nameDefinitionIdx := getDocIdx(doc.Content, "name")
	if nameDefinitionIdx != -1 {
		contentCopy = append(contentCopy, doc.Content[nameDefinitionIdx], doc.Content[nameDefinitionIdx+1])
		nodes = append(nodes, nameDefinitionIdx, nameDefinitionIdx+1)
	}

	buildDefinitionIdx := getDocIdx(doc.Content, "build")
	if buildDefinitionIdx != -1 {
		contentCopy = append(contentCopy, doc.Content[buildDefinitionIdx], doc.Content[buildDefinitionIdx+1])
		nodes = append(nodes, buildDefinitionIdx, buildDefinitionIdx+1)
	} else {
		whereToInject := getDocIdx(contentCopy, "name")
		contentCopy[whereToInject].FootComment = fmt.Sprintf("%s\n%s", buildHeadComment, buildExample)
	}

	deployDefinitionIdx := getDocIdx(doc.Content, "deploy")
	if deployDefinitionIdx != -1 {
		contentCopy = append(contentCopy, doc.Content[deployDefinitionIdx], doc.Content[deployDefinitionIdx+1])
		nodes = append(nodes, deployDefinitionIdx, deployDefinitionIdx+1)
	}

	dependenciesDefinitionIdx := getDocIdx(doc.Content, "dependencies")
	if dependenciesDefinitionIdx != -1 {
		contentCopy = append(contentCopy, doc.Content[dependenciesDefinitionIdx], doc.Content[dependenciesDefinitionIdx+1])
		nodes = append(nodes, dependenciesDefinitionIdx, dependenciesDefinitionIdx+1)
	} else {
		whereToInject := getDocIdx(contentCopy, "deploy")
		contentCopy[whereToInject].FootComment = fmt.Sprintf("%s\n%s", dependenciesHeadComment, dependenciesExample)
	}

	devDefinitionIdx := getDocIdx(doc.Content, "dev")
	if devDefinitionIdx != -1 {
		contentCopy = append(contentCopy, doc.Content[devDefinitionIdx], doc.Content[devDefinitionIdx+1])
		nodes = append(nodes, devDefinitionIdx, devDefinitionIdx+1)
	}

	// We need to inject all the other fields remaining
	if len(doc.Content) != len(contentCopy) {
		for idx := range doc.Content {
			found := false
			for _, copyIdx := range nodes {
				if copyIdx == idx {
					found = true
					break
				}
			}
			if found {
				continue
			}
			contentCopy = append(contentCopy, doc.Content[idx])
		}
	}
	doc.Content = contentCopy
	return doc
}

func getDocIdx(contents []*yaml3.Node, value string) int {
	for idx, node := range contents {
		if node.Value == value {
			return idx
		}
	}
	return -1
}
func addEmptyLineBetweenSections(out []byte) []byte {
	prevLineCommented := false
	newYaml := ""
	for _, line := range strings.Split(string(out), "\n") {
		isLineCommented := strings.HasPrefix(strings.TrimSpace(line), "#")
		if !prevLineCommented && isLineCommented {
			newYaml += "\n"
		}
		newYaml += line + "\n"
		prevLineCommented = isLineCommented
	}
	return []byte(newYaml)
}
