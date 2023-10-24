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

package model

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/a8m/envsubst"
	"github.com/a8m/envsubst/parse"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/discovery"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/spf13/afero"
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
	buildSvcEnvVars   = "You can use the following env vars to refer to this image in your deploy commands:\n - OKTETO_BUILD_%s_REGISTRY: image registry\n - OKTETO_BUILD_%s_REPOSITORY: image repo\n - OKTETO_BUILD_%s_IMAGE: image name\n - OKTETO_BUILD_%s_SHA: image tag sha256"
	deployHeadComment = "The deploy section defines how to deploy your development environment\nMore info: https://www.okteto.com/docs/reference/manifest/#deploy"
	deployExample     = `deploy:
  commands:
  - name: Deploy
    command: echo 'Replace this line with the proper 'helm' or 'kubectl' commands to deploy your development environment'`
	devHeadComment = "The dev section defines how to activate a development container\nMore info: https://www.okteto.com/docs/reference/manifest/#dev"
	devExample     = `dev:
  sample:
    image: okteto/dev:latest
    command: bash
    workdir: /usr/src/app
    sync:
      - .:/usr/src/app
    environment:
      - name=$USER
    forward:
      - 8080:80`
	dependenciesHeadComment = "The dependencies section defines other git repositories to be deployed as part of your development environment\nMore info: https://www.okteto.com/docs/reference/manifest/#dependencies"
	dependenciesExample     = `dependencies:
  - https://github.com/okteto/sample`

	// FakeCommand prints into terminal a fake command
	FakeCommand = "echo 'Replace this line with the proper 'helm' or 'kubectl' commands to deploy your development environment'"
)

var (
	priorityOrder = []string{"dev", "dependencies", "deploy", "build", "name"}
)

// Manifest represents an okteto manifest
type Manifest struct {
	Name          string                                   `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace     string                                   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Context       string                                   `json:"context,omitempty" yaml:"context,omitempty"`
	Icon          string                                   `json:"icon,omitempty" yaml:"icon,omitempty"`
	Deploy        *DeployInfo                              `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	Dev           ManifestDevs                             `json:"dev,omitempty" yaml:"dev,omitempty"`
	Destroy       *DestroyInfo                             `json:"destroy,omitempty" yaml:"destroy,omitempty"`
	Build         ManifestBuild                            `json:"build,omitempty" yaml:"build,omitempty"`
	Dependencies  ManifestDependencies                     `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	GlobalForward []forward.GlobalForward                  `json:"forward,omitempty" yaml:"forward,omitempty"`
	External      externalresource.ExternalResourceSection `json:"external,omitempty" yaml:"external,omitempty"`

	Type     Archetype `json:"-" yaml:"-"`
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
		Dev:           map[string]*Dev{},
		Build:         map[string]*BuildInfo{},
		Dependencies:  map[string]*Dependency{},
		Deploy:        &DeployInfo{},
		GlobalForward: []forward.GlobalForward{},
		External:      externalresource.ExternalResourceSection{},
	}
}

// NewManifestFromStack creates a new manifest from a stack struct
func NewManifestFromStack(stack *Stack) *Manifest {
	stackPaths := ComposeInfoList{}
	for _, path := range stack.Paths {
		stackPaths = append(stackPaths, ComposeInfo{
			File: path,
		})
	}
	stackManifest := &Manifest{
		Type:      StackType,
		Name:      stack.Name,
		Namespace: stack.Namespace,
		Deploy: &DeployInfo{
			ComposeSection: &ComposeSectionInfo{
				ComposesInfo: stackPaths,
				Stack:        stack,
			},
		},
		Dev:   ManifestDevs{},
		Build: ManifestBuild{},
		IsV2:  true,
	}
	cwd, err := os.Getwd()
	if err != nil {
		oktetoLog.Fail("Can not find cwd: %s", err)
		return nil
	}
	stackManifest, err = stackManifest.InferFromStack(cwd)
	if err != nil {
		oktetoLog.Fail("Can not convert stack to Manifestv2: %s", err)
		return nil
	}
	return stackManifest
}

// NewManifestFromDev creates a manifest from a dev
func NewManifestFromDev(dev *Dev) *Manifest {
	manifest := NewManifest()
	name, err := ExpandEnv(dev.Name, true)
	if err != nil {
		oktetoLog.Infof("could not expand dev name '%s'", dev.Name)
		name = dev.Name
	}
	manifest.Dev[name] = dev
	return manifest
}

// DeployInfo represents what must be deployed for the app to work
type DeployInfo struct {
	Image          string              `json:"image,omitempty" yaml:"image,omitempty"`
	Commands       []DeployCommand     `json:"commands,omitempty" yaml:"commands,omitempty"`
	ComposeSection *ComposeSectionInfo `json:"compose,omitempty" yaml:"compose,omitempty"`
	Endpoints      EndpointSpec        `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	Divert         *DivertDeploy       `json:"divert,omitempty" yaml:"divert,omitempty"`
	Remote         bool                `json:"remote,omitempty" yaml:"remote,omitempty"`
}

// DestroyInfo represents what must be destroyed for the app
type DestroyInfo struct {
	Image    string          `json:"image,omitempty" yaml:"image,omitempty"`
	Commands []DeployCommand `json:"commands,omitempty" yaml:"commands,omitempty"`
	Remote   bool            `json:"remote,omitempty" yaml:"remote,omitempty"`
}

// DivertDeploy represents information about the deploy divert configuration
type DivertDeploy struct {
	Driver               string                 `json:"driver,omitempty" yaml:"driver,omitempty"`
	Namespace            string                 `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	DeprecatedService    string                 `json:"service,omitempty" yaml:"service,omitempty"`
	DeprecatedPort       int                    `json:"port,omitempty" yaml:"port,omitempty"`
	DeprecatedDeployment string                 `json:"deployment,omitempty" yaml:"deployment,omitempty"`
	VirtualServices      []DivertVirtualService `json:"virtualServices,omitempty" yaml:"virtualServices,omitempty"`
	Hosts                []DivertHost           `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// DivertVirtualService represents a virtual service in a namespace to be diverted
type DivertVirtualService struct {
	Name      string   `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Routes    []string `json:"routes,omitempty" yaml:"routes,omitempty"`
}

// DivertHost represents a host from a virtual service in a namespace to be diverted
type DivertHost struct {
	VirtualService string `json:"virtualService,omitempty" yaml:"virtualService,omitempty"`
	Namespace      string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// ComposeSectionInfo represents information about compose file
type ComposeSectionInfo struct {
	ComposesInfo ComposeInfoList `json:"manifest,omitempty" yaml:"manifest,omitempty"`
	Stack        *Stack          `json:"-" yaml:"-"`
}

type ComposeInfoList []ComposeInfo

type ComposeInfo struct {
	File             string           `json:"file,omitempty" yaml:"file,omitempty"`
	ServicesToDeploy ServicesToDeploy `json:"services,omitempty" yaml:"services,omitempty"`
}

type ServicesToDeploy []string

// DeployCommand represents a command to be executed
type DeployCommand struct {
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Command string `json:"command,omitempty" yaml:"command,omitempty"`
}

// NewDeployInfo creates a deploy Info
func NewDeployInfo() *DeployInfo {
	return &DeployInfo{
		Commands: []DeployCommand{},
	}
}

// NewDestroyInfo creates a destroy Info
func NewDestroyInfo() *DestroyInfo {
	return &DestroyInfo{
		Commands: []DeployCommand{},
	}
}

func getManifestFromOktetoFile(cwd string) (*Manifest, error) {
	manifestPath, err := discovery.GetOktetoManifestPath(cwd)
	if err != nil {
		return nil, err
	}
	oktetoLog.Infof("Found okteto manifest file on path: %s", manifestPath)
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Found okteto manifest on %s", manifestPath)
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Unmarshalling manifest...")
	devManifest, err := getManifestFromFile(cwd, manifestPath)
	if err != nil {
		return nil, err
	}
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Okteto manifest unmarshalled successfully")
	return devManifest, nil
}

func getManifestFromDevFilePath(cwd, manifestPath string) (*Manifest, error) {
	if manifestPath != "" && !filepath.IsAbs(manifestPath) {
		manifestPath = filepath.Join(cwd, manifestPath)
	}
	if manifestPath != "" && filesystem.FileExistsAndNotDir(manifestPath) {
		return getManifestFromFile(cwd, manifestPath)
	}

	return nil, discovery.ErrOktetoManifestNotFound
}

// GetManifestV1 gets a manifest from a path or search for the files to generate it
func GetManifestV1(manifestPath string) (*Manifest, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	manifest, err := getManifestFromDevFilePath(cwd, manifestPath)
	if err != nil {
		if !errors.Is(err, discovery.ErrOktetoManifestNotFound) {
			return nil, err
		}
	}

	if manifest != nil {
		return manifest, nil
	}

	if manifestPath != "" && pathExistsAndDir(manifestPath) {
		cwd = manifestPath
	}

	manifest, err = getManifestFromOktetoFile(cwd)
	if err != nil {
		return nil, err
	}

	return manifest, nil
}

// GetManifestV2 gets a manifest from a path or search for the files to generate it
func GetManifestV2(manifestPath string) (*Manifest, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	manifest, err := getManifestFromDevFilePath(cwd, manifestPath)
	if err != nil {
		if !errors.Is(err, discovery.ErrOktetoManifestNotFound) {
			return nil, err
		}
	}

	if manifest != nil {
		return manifest, nil
	}

	if manifestPath != "" && pathExistsAndDir(manifestPath) {
		cwd = manifestPath
	}

	manifest, err = getManifestFromOktetoFile(cwd)
	if err != nil {
		if !errors.Is(err, discovery.ErrOktetoManifestNotFound) {
			return nil, err
		}
	}

	if manifest != nil && manifest.IsV2 {
		return manifest, nil
	}

	inferredManifest, err := GetInferredManifest(cwd)
	if err != nil {
		return nil, err
	}
	if inferredManifest != nil {
		if inferredManifest.Type == StackType {
			inferredManifest, err = inferredManifest.InferFromStack(cwd)
			if err != nil {
				return nil, err
			}
			if manifest != nil && manifest.Deploy != nil && manifest.Deploy.Endpoints != nil {
				inferredManifest.Deploy.ComposeSection.Stack.Endpoints = manifest.Deploy.Endpoints
			}
			if inferredManifest.Deploy.ComposeSection.Stack.Name != "" {
				inferredManifest.Name = inferredManifest.Deploy.ComposeSection.Stack.Name
			}
		}
		if manifest != nil {
			inferredManifest.mergeWithOktetoManifest(manifest)
		}

		if len(inferredManifest.Manifest) == 0 {
			bytes, err := yaml.Marshal(inferredManifest)
			if err != nil {
				return nil, err
			}
			inferredManifest.Manifest = bytes
		}
		return inferredManifest, nil
	}

	if manifest != nil {
		manifest.Type = OktetoType
		manifest.Deploy = &DeployInfo{
			Commands: []DeployCommand{
				{
					Name:    "okteto push",
					Command: "okteto push",
				},
			},
		}
		return manifest, nil
	}
	return nil, discovery.ErrOktetoManifestNotFound
}

// getManifestFromFile retrieves the manifest from a given file, okteto manifest or docker-compose
func getManifestFromFile(cwd, manifestPath string) (*Manifest, error) {
	devManifest, err := getOktetoManifest(manifestPath)
	if err != nil {
		oktetoLog.Info("devManifest err, fallback to stack unmarshall")
		stackManifest := &Manifest{
			Type: StackType,
			Deploy: &DeployInfo{
				ComposeSection: &ComposeSectionInfo{
					ComposesInfo: []ComposeInfo{
						{File: manifestPath},
					},
				},
			},
			Dev:   ManifestDevs{},
			Build: ManifestBuild{},
			IsV2:  true,
		}
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Unmarshalling compose...")

		var composeFiles []string
		for _, composeInfo := range stackManifest.Deploy.ComposeSection.ComposesInfo {
			composeFiles = append(composeFiles, composeInfo.File)
		}
		// We need to ensure that LoadStack has false because we don't want to expand env vars
		s, stackErr := LoadStack("", composeFiles, false)
		if stackErr != nil {
			// if err is from validation, then return the stackErr
			if errors.Is(stackErr, errDependsOn) {
				return nil, stackErr
			}

			if errors.Is(stackErr, oktetoErrors.ErrServiceEmpty) {
				return nil, stackErr
			}
			// if not return original manifest err
			return nil, err
		}
		stackManifest.Deploy.ComposeSection.Stack = s
		if stackManifest.Deploy.ComposeSection.Stack.Name != "" {
			stackManifest.Name = stackManifest.Deploy.ComposeSection.Stack.Name
		}
		stackManifest, err = stackManifest.InferFromStack(cwd)
		if err != nil {
			return nil, err
		}
		stackManifest.Manifest = s.Manifest
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Okteto compose unmarshalled successfully")
		return stackManifest, nil
	}
	if devManifest.IsV2 {
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Okteto manifest v2 unmarshalled successfully")
		devManifest.Type = OktetoManifestType
		if devManifest.Deploy != nil && devManifest.Deploy.ComposeSection != nil && len(devManifest.Deploy.ComposeSection.ComposesInfo) > 0 {
			var stackFiles []string
			for _, composeInfo := range devManifest.Deploy.ComposeSection.ComposesInfo {
				stackFiles = append(stackFiles, composeInfo.File)
			}
			// LoadStack should perform validation of the stack read from the file on compose section
			// We need to ensure that LoadStack has false because we don't want to expand env vars
			s, err := LoadStack("", stackFiles, false)
			if err != nil {
				return nil, err
			}
			devManifest.Deploy.ComposeSection.Stack = s
			devManifest, err = devManifest.InferFromStack(cwd)
			if devManifest.Deploy.Endpoints != nil {
				s.Endpoints = devManifest.Deploy.Endpoints
			}
			if err != nil {
				return nil, err
			}
		}
		return devManifest, nil
	} else {
		devManifest.setManifestDefaultsFromDev()
	}
	return devManifest, nil

}

// GetInferredManifest infers the manifest from a directory
func GetInferredManifest(cwd string) (*Manifest, error) {
	pipelinePath, err := discovery.GetOktetoPipelinePath(cwd)
	if err == nil {
		oktetoLog.Infof("Found pipeline on: %s", pipelinePath)
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Found okteto pipeline manifest on %s", pipelinePath)
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Unmarshalling pipeline manifest...")
		pipelineManifest, err := GetManifestV2(pipelinePath)
		if err != nil {
			return nil, err
		}
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Okteto pipeline manifest unmarshalled successfully")
		pipelineManifest.Type = PipelineType
		return pipelineManifest, nil
	}

	composePath, err := discovery.GetComposePath(cwd)
	if err == nil {
		oktetoLog.Infof("Found okteto compose")
		stackPath, err := filepath.Rel(cwd, composePath)
		if err != nil {
			return nil, err
		}
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Found okteto compose manifest on %s", stackPath)
		stackManifest := &Manifest{
			Type: StackType,
			Deploy: &DeployInfo{
				ComposeSection: &ComposeSectionInfo{
					ComposesInfo: []ComposeInfo{
						{File: stackPath},
					},
				},
			},
			Dev:   ManifestDevs{},
			Build: ManifestBuild{},
			IsV2:  true,
		}
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Unmarshalling compose...")
		var stackFiles []string
		for _, composeInfo := range stackManifest.Deploy.ComposeSection.ComposesInfo {
			stackFiles = append(stackFiles, composeInfo.File)
		}
		s, err := LoadStack("", stackFiles, true)
		if err != nil {
			return nil, err
		}
		stackManifest.Deploy.ComposeSection.Stack = s
		stackManifest.Manifest = s.Manifest
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Okteto compose unmarshalled successfully")

		return stackManifest, nil
	}

	chartPath, err := discovery.GetHelmChartPath(cwd)
	if err == nil {
		oktetoLog.Infof("Found chart")
		chartPath, err := filepath.Rel(cwd, chartPath)
		if err != nil {
			return nil, err
		}
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Found helm chart on %s", chartPath)
		tags := inferHelmTags(chartPath)
		deployHelm := fmt.Sprintf("helm upgrade --install ${%s} %s %s", constants.OktetoAutodiscoveryReleaseName, chartPath, tags)
		chartManifest := &Manifest{
			Type: ChartType,
			Deploy: &DeployInfo{
				Commands: []DeployCommand{
					{
						Name:    deployHelm,
						Command: deployHelm,
					},
				},
			},
			Dev:   ManifestDevs{},
			Build: ManifestBuild{},
		}
		return chartManifest, nil
	}

	k8sManifestPath, err := discovery.GetK8sManifestPath(cwd)
	if err == nil {
		oktetoLog.Infof("Found kubernetes manifests")
		manifestPath, err := filepath.Rel(cwd, k8sManifestPath)
		if err != nil {
			return nil, err
		}
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Found kubernetes manifest on %s", manifestPath)
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
			Dev:   ManifestDevs{},
			Build: ManifestBuild{},
		}
		return k8sManifest, nil
	}

	return nil, oktetoErrors.ErrCouldNotInferAnyManifest
}

func inferHelmTags(path string) string {
	valuesPath := filepath.Join(path, "values.yaml")
	if _, err := os.Stat(valuesPath); err != nil {
		oktetoLog.Info("chart values not found")
		return ""
	}
	type valuesImages struct {
		Image string `yaml:"image,omitempty"`
	}

	b, err := os.ReadFile(valuesPath)
	if err != nil {
		oktetoLog.Info("could not read file values")
		return ""
	}
	tags := map[string]*valuesImages{}
	if err := yaml.Unmarshal(b, tags); err != nil {
		oktetoLog.Info("could not parse values image tags")
		return ""
	}
	result := ""
	for svcName, image := range tags {
		if image == nil {
			continue
		}
		if image.Image != "" {
			result += fmt.Sprintf(" --set %s.image=${OKTETO_BUILD_%s_IMAGE}", svcName, strings.ToUpper(svcName))
			os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_IMAGE", strings.ToUpper(svcName)), image.Image)
		}
	}
	return result
}

// getOktetoManifest returns an okteto object from a given file
func getOktetoManifest(devPath string) (*Manifest, error) {
	b, err := os.ReadFile(devPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, discovery.ErrOktetoManifestNotFound
		}
		return nil, err
	}

	if isEmptyManifestFile(b) {
		return nil, fmt.Errorf("%s: %w", oktetoErrors.ErrInvalidManifest, oktetoErrors.ErrEmptyManifest)
	}

	manifest, err := Read(b)
	if err != nil {
		if errors.Is(err, oktetoErrors.ErrNotManifestContentDetected) {
			return nil, err
		}
		return nil, newManifestFriendlyError(err)
	}

	ef := externalresource.ERFilesystemManager{
		Fs: afero.NewOsFs(),
	}

	for name, external := range manifest.External {
		external.SetDefaults(name)
		ef.ExternalResource = *external
		err := ef.LoadMarkdownContent(devPath)
		if err != nil {
			oktetoLog.Infof("error loading external resource %s: %s", name, err.Error())
		}
	}

	for _, dev := range manifest.Dev {

		if err := dev.loadAbsPaths(devPath); err != nil {
			return nil, err
		}

		if err := dev.expandEnvFiles(); err != nil {
			return nil, err
		}

		dev.computeParentSyncFolder()
	}

	return manifest, nil
}

func isEmptyManifestFile(bytes []byte) bool {
	if bytes == nil || strings.TrimSpace(string(bytes)) == "" {
		return true
	}
	return false
}

// Read reads an okteto manifests
func Read(bytes []byte) (*Manifest, error) {
	manifest := NewManifest()

	if bytes != nil {
		if err := yaml.UnmarshalStrict(bytes, manifest); err != nil {
			if err := yaml.Unmarshal(bytes, manifest); err == nil {
				if reflect.DeepEqual(manifest, NewManifest()) {
					return nil, oktetoErrors.ErrNotManifestContentDetected
				}
			}
			return nil, err
		}
	}

	hasShownWarning := false
	for _, dev := range manifest.Dev {
		if (dev.Image.Context != "" || dev.Image.Dockerfile != "") && !hasShownWarning {
			hasShownWarning = true
			oktetoLog.Yellow(`The 'image' extended syntax is deprecated and will be removed in a future version. Define the images you want to build in the 'build' section of your manifest. More info at https://www.okteto.com/docs/reference/manifest/#build"`)
		}
	}

	if err := manifest.setDefaults(); err != nil {
		return nil, err
	}

	if err := manifest.validate(); err != nil {
		return nil, err
	}
	manifest.Manifest = bytes
	manifest.Type = OktetoManifestType
	return manifest, nil
}

func (m *Manifest) validate() error {
	if err := m.Build.validate(); err != nil {
		return err
	}
	return m.validateDivert()
}

func (b *ManifestBuild) validate() error {
	cycle := getDependentCyclic(b.toGraph())
	if len(cycle) == 1 { // depends on the same node
		return fmt.Errorf("manifest build validation failed: image '%s' is referenced on its dependencies", cycle[0])
	} else if len(cycle) > 1 {
		svcsDependents := fmt.Sprintf("%s and %s", strings.Join(cycle[:len(cycle)-1], ", "), cycle[len(cycle)-1])
		return fmt.Errorf("manifest validation failed: cyclic dependendecy found between %s", svcsDependents)
	}
	return nil
}

// GetSvcsToBuildFromList returns the builds from a list and all its
func (b *ManifestBuild) GetSvcsToBuildFromList(toBuild []string) []string {
	initialSvcsToBuild := toBuild
	svcsToBuildWithDependencies := getDependentNodes(b.toGraph(), toBuild)
	if len(initialSvcsToBuild) != len(svcsToBuildWithDependencies) {
		dependantBuildImages := getListDiff(initialSvcsToBuild, svcsToBuildWithDependencies)
		oktetoLog.Warning("The following build images need to be built because of dependencies: [%s]", strings.Join(dependantBuildImages, ", "))
	}
	return svcsToBuildWithDependencies
}

func (b ManifestBuild) toGraph() graph {
	g := graph{}
	for k, v := range b {
		g[k] = v.DependsOn
	}
	return g
}

// SanitizeSvcNames sanitize service names in 'dev', 'build' and 'global forward' sections
func (m *Manifest) SanitizeSvcNames() error {
	sanitizedServicesNames := make(map[string]string)
	for devKey, dev := range m.Dev {
		if shouldBeSanitized(devKey) {
			sanitizedSvcName := sanitizeName(devKey)
			if _, ok := m.Dev[sanitizedSvcName]; ok {
				return fmt.Errorf("could not sanitize '%s'. Service with name '%s' already exists", devKey, sanitizedSvcName)
			}
			dev.Name = sanitizedSvcName
			m.Dev[sanitizedSvcName] = dev
			sanitizedServicesNames[devKey] = sanitizedSvcName
			delete(m.Dev, devKey)
		}
	}

	for buildKey, build := range m.Build {
		if shouldBeSanitized(buildKey) {
			sanitizedSvcName := sanitizeName(buildKey)
			if _, ok := m.Build[sanitizedSvcName]; ok {
				return fmt.Errorf("could not sanitize '%s'. Service with name '%s' already exists", buildKey, sanitizedSvcName)
			}
			m.Build[sanitizedSvcName] = build
			sanitizedServicesNames[buildKey] = sanitizedSvcName
			delete(m.Build, buildKey)
		}
	}

	for idx := range m.GlobalForward {
		gfServiceName := m.GlobalForward[idx].ServiceName
		if shouldBeSanitized(gfServiceName) {
			sanitizedSvcName := sanitizeName(gfServiceName)
			sanitizedServicesNames[gfServiceName] = sanitizedSvcName
			m.GlobalForward[idx].ServiceName = sanitizedSvcName
		}
	}

	for previousName, newName := range sanitizedServicesNames {
		oktetoLog.Warning("Service '%s' specified in okteto manifest has been sanitized into '%s'.", previousName, newName)
	}

	return nil
}

func (m *Manifest) validateDivert() error {
	if m.Deploy == nil {
		return nil
	}
	if m.Deploy.Divert == nil {
		return nil
	}

	switch m.Deploy.Divert.Driver {
	case constants.OktetoDivertWeaverDriver:
		if m.Deploy.Divert.Namespace == "" {
			return fmt.Errorf("the field 'deploy.divert.namespace' is mandatory")
		}
		if len(m.Deploy.Divert.VirtualServices) > 0 {
			return fmt.Errorf("the field 'deploy.divert.virtualServices' is not supported with the weaver driver")
		}
		if len(m.Deploy.Divert.Hosts) > 0 {
			return fmt.Errorf("the field 'deploy.divert.host' is not supported with the weaver driver")
		}
	case constants.OktetoDivertIstioDriver:
		if m.Deploy.Divert.DeprecatedService != "" {
			return fmt.Errorf("the field 'deploy.divert.service' is not supported with the istio driver")
		}
		if m.Deploy.Divert.Namespace != "" {
			return fmt.Errorf("the field 'deploy.divert.namespace' is not supported with the istio driver")
		}
		if len(m.Deploy.Divert.VirtualServices) == 0 {
			return fmt.Errorf("the field 'deploy.divert.virtualServices' is mandatory")
		}
		for i := range m.Deploy.Divert.VirtualServices {
			if m.Deploy.Divert.VirtualServices[i].Name == "" {
				return fmt.Errorf("the field 'deploy.divert.virtualServices[%d].name' is mandatory", i)
			}
			if m.Deploy.Divert.VirtualServices[i].Namespace == "" {
				return fmt.Errorf("the field 'deploy.divert.virtualServices[%d].namespace' is mandatory", i)
			}
		}
		for i := range m.Deploy.Divert.Hosts {
			if m.Deploy.Divert.Hosts[i].VirtualService == "" {
				return fmt.Errorf("the field 'deploy.divert.hosts[%d].virtualService' is mandatory", i)
			}
			if m.Deploy.Divert.Hosts[i].Namespace == "" {
				return fmt.Errorf("the field 'deploy.divert.hosts[%d].namespace' is mandatory", i)
			}
		}
	default:
		return fmt.Errorf("the divert driver '%s' isn't supported", m.Deploy.Divert.Driver)
	}
	return nil
}

func (m *Manifest) setDefaults() error {
	if m.Deploy != nil && m.Deploy.Divert != nil {
		var err error
		if m.Deploy.Divert.Driver == "" {
			m.Deploy.Divert.Driver = constants.OktetoDivertWeaverDriver
		}
		m.Deploy.Divert.Namespace, err = ExpandEnv(m.Deploy.Divert.Namespace, false)
		if err != nil {
			return err
		}
		for i := range m.Deploy.Divert.Hosts {
			m.Deploy.Divert.Hosts[i].VirtualService, err = ExpandEnv(m.Deploy.Divert.Hosts[i].VirtualService, false)
			if err != nil {
				return err
			}
			m.Deploy.Divert.Hosts[i].Namespace, err = ExpandEnv(m.Deploy.Divert.Hosts[i].Namespace, false)
			if err != nil {
				return err
			}
		}
	}
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

		d.translateDeprecatedMetadataFields()

		sort.SliceStable(d.Forward, func(i, j int) bool {
			return d.Forward[i].Less(&d.Forward[j])
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

		if !(b.Image != "" && len(b.VolumesToInclude) > 0 && b.Dockerfile == "") {
			b.setBuildDefaults()
		}
	}

	if m.Destroy == nil {
		m.Destroy = &DestroyInfo{}
	}

	return nil
}

func (m *Manifest) mergeWithOktetoManifest(other *Manifest) {
	if len(m.Dev) == 0 && len(other.Dev) != 0 {
		m.Dev = other.Dev
	}
}

// ExpandEnvVars expands env vars to be set on the manifest
func (manifest *Manifest) ExpandEnvVars() error {
	var err error
	if manifest.Deploy != nil {
		if manifest.Deploy.Image != "" {
			manifest.Deploy.Image, err = envsubst.String(manifest.Deploy.Image)
			if err != nil {
				return errors.New("could not parse env vars for an image used for remote deploy")
			}
		}
		if manifest.Deploy.ComposeSection != nil && manifest.Deploy.ComposeSection.Stack != nil {
			var stackFiles []string
			for _, composeInfo := range manifest.Deploy.ComposeSection.ComposesInfo {
				stackFiles = append(stackFiles, composeInfo.File)
			}
			s, err := LoadStack("", stackFiles, true)
			if err != nil {
				oktetoLog.Infof("Could not reload stack manifest: %s", err)
				s = manifest.Deploy.ComposeSection.Stack
			}
			s.Namespace = manifest.Deploy.ComposeSection.Stack.Namespace
			s.Context = manifest.Deploy.ComposeSection.Stack.Context
			for svcName, svc := range s.Services {
				if _, ok := manifest.Build[svcName]; svc.Build == nil && len(svc.VolumeMounts) == 0 && !ok {
					continue
				}
				tag := fmt.Sprintf("${OKTETO_BUILD_%s_IMAGE}", strings.ToUpper(strings.ReplaceAll(svcName, "-", "_")))
				expandedTag, err := ExpandEnv(tag, true)
				if err != nil {
					return err
				}
				if expandedTag != "" {
					svc.Image = expandedTag
				}
			}
			manifest.Deploy.ComposeSection.Stack = s
			if len(manifest.Deploy.Endpoints) > 0 {
				if len(manifest.Deploy.ComposeSection.Stack.Endpoints) > 0 {
					oktetoLog.Warning("Endpoints are defined in the stack and in the manifest. The endpoints defined in the manifest will be used")
				}
				manifest.Deploy.ComposeSection.Stack.Endpoints = manifest.Deploy.Endpoints
			}
			cwd, err := os.Getwd()
			if err != nil {
				oktetoLog.Info("could not detect working directory")
			}
			manifest, err = manifest.InferFromStack(cwd)
			if err != nil {
				return err
			}
		}
	}
	if manifest.Destroy != nil {
		if manifest.Destroy.Image != "" {
			manifest.Destroy.Image, err = ExpandEnv(manifest.Destroy.Image, true)
			if err != nil {
				return err
			}
		}
	}

	for devName, devInfo := range manifest.Dev {
		if _, ok := manifest.Build[devName]; ok && devInfo.Image == nil && devInfo.Autocreate {
			devInfo.Image = &BuildInfo{
				Name: fmt.Sprintf("${OKTETO_BUILD_%s_IMAGE}", strings.ToUpper(strings.ReplaceAll(devName, "-", "_"))),
			}
		}
		if devInfo.Image != nil {
			devInfo.Image.Name, err = ExpandEnv(devInfo.Image.Name, false)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Dependency represents a dependency object at the manifest
type Dependency struct {
	Repository   string        `json:"repository" yaml:"repository"`
	ManifestPath string        `json:"manifest,omitempty" yaml:"manifest,omitempty"`
	Branch       string        `json:"branch,omitempty" yaml:"branch,omitempty"`
	Variables    Environment   `json:"variables,omitempty" yaml:"variables,omitempty"`
	Wait         bool          `json:"wait,omitempty" yaml:"wait,omitempty"`
	Timeout      time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Namespace    string        `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// GetTimeout returns dependency.Timeout if it's set or the one passed as arg if it's not
func (d *Dependency) GetTimeout(defaultTimeout time.Duration) time.Duration {
	if d.Timeout != 0 {
		return d.Timeout
	}
	return defaultTimeout
}

// ExpandVars sets dependencies values if values fits with list params
func (d *Dependency) ExpandVars(variables []string) error {
	parser := parse.New("string", append(os.Environ(), variables...), &parse.Restrictions{})

	expandedBranch, err := parser.Parse(d.Branch)
	if err != nil {
		return fmt.Errorf("error expanding 'branch': %w", err)
	}
	if expandedBranch != "" {
		d.Branch = expandedBranch
	}

	expandedRepository, err := parser.Parse(d.Repository)
	if err != nil {
		return fmt.Errorf("error expanding 'repository': %w", err)
	}
	if expandedRepository != "" {
		d.Repository = expandedRepository
	}

	expandedManifestPath, err := parser.Parse(d.ManifestPath)
	if err != nil {
		return fmt.Errorf("error expanding 'manifest': %w", err)
	}
	if expandedManifestPath != "" {
		d.ManifestPath = expandedManifestPath
	}

	expandedNamespace, err := parser.Parse(d.Namespace)
	if err != nil {
		return fmt.Errorf("error expanding 'namespace': %w", err)
	}
	if expandedNamespace != "" {
		d.Namespace = expandedNamespace
	}

	expandedVariables := Environment{}
	for _, v := range d.Variables {
		expandedVarName, err := parser.Parse(v.Name)
		if err != nil {
			return fmt.Errorf("error expanding variable name: %w", err)
		}
		if expandedVarName != "" {
			v.Name = expandedVarName
		}

		expandedVarValue, err := parser.Parse(v.Value)
		if err != nil {
			return fmt.Errorf("error expanding variable value: %w", err)
		}
		if expandedVarValue != "" {
			v.Value = expandedVarValue
		}

		expandedVariables = append(expandedVariables, EnvVar{
			Name:  v.Name,
			Value: v.Value,
		})
	}
	d.Variables = expandedVariables

	return nil
}

// InferFromStack infers data from a stackfile
func (m *Manifest) InferFromStack(cwd string) (*Manifest, error) {
	for svcName, svcInfo := range m.Deploy.ComposeSection.Stack.Services {
		d, err := svcInfo.ToDev(svcName)
		if err != nil {
			return nil, err
		}

		d.parentSyncFolder = cwd

		if _, ok := m.Dev[svcName]; !ok && len(d.Sync.Folders) > 0 {
			m.Dev[svcName] = d
		}

		if svcInfo.Build == nil && len(svcInfo.VolumeMounts) == 0 {
			continue
		}

		buildInfo := svcInfo.Build

		switch {
		case buildInfo != nil && len(svcInfo.VolumeMounts) > 0:
			if svcInfo.Image != "" {
				buildInfo.Image = svcInfo.Image
			}
			buildInfo.VolumesToInclude = svcInfo.VolumeMounts
		case buildInfo != nil:
			if svcInfo.Image != "" {
				buildInfo.Image = svcInfo.Image
			}
		case len(svcInfo.VolumeMounts) > 0:
			buildInfo = &BuildInfo{
				Image:            svcInfo.Image,
				VolumesToInclude: svcInfo.VolumeMounts,
			}
		default:
			oktetoLog.Infof("could not build service %s, due to not having Dockerfile defined or volumes to include", svcName)
		}

		for idx, volume := range buildInfo.VolumesToInclude {
			localPath := volume.LocalPath
			if filepath.IsAbs(localPath) {
				localPath, err = filepath.Rel(buildInfo.Context, volume.LocalPath)
				if err != nil {
					localPath, err = filepath.Rel(cwd, volume.LocalPath)
					if err != nil {
						oktetoLog.Info("can not find svc[%s].build.volumes to include relative to svc[%s].build.context", svcName, svcName)
					}
				}
			}
			volume.LocalPath = localPath
			buildInfo.VolumesToInclude[idx] = volume
		}
		buildInfo.Context, err = filepath.Rel(cwd, buildInfo.Context)
		if err != nil {
			oktetoLog.Infof("can not make svc[%s].build.context relative to cwd", svcName)
		}
		contextAbs := filepath.Join(cwd, buildInfo.Context)
		buildInfo.Dockerfile, err = filepath.Rel(contextAbs, buildInfo.Dockerfile)
		if err != nil {
			oktetoLog.Infof("can not make svc[%s].build.dockerfile relative to cwd", svcName)
		}
		if _, ok := m.Build[svcName]; !ok {
			m.Build[svcName] = buildInfo
		}
	}
	if err := m.setDefaults(); err != nil {
		oktetoLog.Infof("failed to set defaults: %s", err)
	}
	return m, nil
}

// WriteToFile writes a manifest to a file with comments to make it easier to understand
func (m *Manifest) WriteToFile(filePath string) error {
	if m.Deploy != nil {
		if len(m.Deploy.Commands) == 0 && m.Deploy.ComposeSection == nil {
			m.Deploy.Commands = []DeployCommand{
				{
					Name:    FakeCommand,
					Command: FakeCommand,
				},
			}
		}
	}
	for _, b := range m.Build {
		b.Name = ""
	}
	for dName, d := range m.Dev {
		d.Name = ""
		d.Context = ""
		d.Namespace = ""
		if d.Image != nil && d.Image.Name != "" {
			d.Image.Context = ""
			d.Image.Dockerfile = ""
		} else {
			if v, ok := m.Build[dName]; ok {
				if v.Image != "" {
					d.Image = &BuildInfo{Name: v.Image}
				} else {
					d.Image = nil
				}
			} else {
				d.Image = nil
			}
		}

		if d.Push != nil && d.Push.Name != "" {
			d.Push.Context = ""
			d.Push.Dockerfile = ""
		} else {
			d.Push = nil
		}
	}
	// Unmarshal with yamlv2 because we have the marshal with yaml v2
	b, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	doc := yaml3.Node{}
	if err := yaml3.Unmarshal(b, &doc); err != nil {
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
				serviceName := strings.ToUpper(strings.ReplaceAll(subsection.Value, "-", "_"))
				subsection.HeadComment = fmt.Sprintf(buildSvcEnvVars, serviceName, serviceName, serviceName, serviceName)
			}
		}
	}

	m.reorderDocFields(&doc)

	buffer := bytes.NewBuffer(nil)
	encoder := yaml3.NewEncoder(buffer)
	encoder.SetIndent(2)

	err = encoder.Encode(doc)
	if err != nil {
		return err
	}
	out := addEmptyLineBetweenSections(buffer.Bytes())
	if err := os.WriteFile(filePath, out, 0600); err != nil {
		oktetoLog.Infof("failed to write stignore file: %s", err)
		return err
	}
	return nil
}

// reorderDocFields orders the manifest to be: name -> build -> deploy -> dependencies -> dev
func (*Manifest) reorderDocFields(doc *yaml3.Node) {
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
		whereToInject := getDocIdxWithPrior(contentCopy, "name")
		contentCopy[whereToInject].FootComment = fmt.Sprintf("%s\n%s", buildHeadComment, buildExample)
	}

	deployDefinitionIdx := getDocIdx(doc.Content, "deploy")
	if deployDefinitionIdx != -1 {
		contentCopy = append(contentCopy, doc.Content[deployDefinitionIdx], doc.Content[deployDefinitionIdx+1])
		nodes = append(nodes, deployDefinitionIdx, deployDefinitionIdx+1)
	} else {
		whereToInject := getDocIdxWithPrior(contentCopy, "build")
		footComment := fmt.Sprintf("%s\n%s", deployHeadComment, deployExample)
		if buildDefinitionIdx == -1 {
			footComment = "\n" + footComment
		}
		if contentCopy[whereToInject].FootComment == "" {
			contentCopy[whereToInject].FootComment = footComment
		} else {
			contentCopy[whereToInject].FootComment += footComment
		}
	}

	dependenciesDefinitionIdx := getDocIdx(doc.Content, "dependencies")
	if dependenciesDefinitionIdx != -1 {
		contentCopy = append(contentCopy, doc.Content[dependenciesDefinitionIdx], doc.Content[dependenciesDefinitionIdx+1])
		nodes = append(nodes, dependenciesDefinitionIdx, dependenciesDefinitionIdx+1)
	} else {
		whereToInject := getDocIdxWithPrior(contentCopy, "deploy")
		footComment := fmt.Sprintf("%s\n%s", dependenciesHeadComment, dependenciesExample)
		if deployDefinitionIdx == -1 {
			footComment = "\n\n" + footComment
		}
		if contentCopy[whereToInject].FootComment == "" {
			contentCopy[whereToInject].FootComment = footComment
		} else {
			contentCopy[whereToInject].FootComment += footComment
		}
	}

	devDefinitionIdx := getDocIdx(doc.Content, "dev")
	if devDefinitionIdx != -1 {
		contentCopy = append(contentCopy, doc.Content[devDefinitionIdx], doc.Content[devDefinitionIdx+1])
		nodes = append(nodes, devDefinitionIdx, devDefinitionIdx+1)
	} else {
		whereToInject := getDocIdxWithPrior(contentCopy, "dependencies")
		footComment := fmt.Sprintf("%s\n%s", devHeadComment, devExample)
		if dependenciesDefinitionIdx == -1 {
			footComment = "\n" + footComment
		}
		if contentCopy[whereToInject].FootComment == "" {
			contentCopy[whereToInject].FootComment = footComment
		} else {
			contentCopy[whereToInject].FootComment += footComment
		}
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
}

func getDocIdxWithPrior(contents []*yaml3.Node, value string) int {
	for idx, node := range contents {
		if node.Value == value {
			return idx
		}
	}
	for idx, val := range priorityOrder {
		if val == value {
			return getDocIdx(contents, priorityOrder[idx+1])
		}
	}
	return -1
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

// IsDeployDefault returns true if the command is empty or if it has the default one
func (m *Manifest) IsDeployDefault() bool {
	if m.Deploy == nil {
		return true
	}
	if len(m.Deploy.Commands) == 1 && m.Deploy.Commands[0].Command == FakeCommand {
		return true
	}
	return false
}

// setManifestDefaultsFromDev sets context and namespace from the dev
func (m *Manifest) setManifestDefaultsFromDev() {
	if len(m.Dev) == 1 {
		for _, devInfo := range m.Dev {
			m.Context = devInfo.Context
			m.Namespace = devInfo.Namespace
		}
	} else {
		oktetoLog.Infof("could not set context and manifest from dev section due to being '%d' devs declared", len(m.Dev))
	}
}

// HasDependencies returns true if the manifest has dependencies
func (m *Manifest) HasDependencies() bool {
	if m.Dependencies == nil {
		return false
	}
	return len(m.Dependencies) > 0
}

// HasDev checks if manifestDevs has a dev name as key
func (d ManifestDevs) HasDev(name string) bool {
	_, ok := d[name]
	return ok
}

// GetDevs returns a list of strings with the keys of devs defined
func (d ManifestDevs) GetDevs() []string {
	devs := []string{}
	for k := range d {
		devs = append(devs, k)
	}
	return devs
}

func (m *Manifest) GetBuildServices() map[string]bool {
	images := map[string]bool{}
	for service := range m.Build {
		images[service] = true
	}
	return images
}

func (m *Manifest) GetStack() *Stack {
	if m.Deploy == nil || m.Deploy.ComposeSection == nil {
		return nil
	}
	return m.Deploy.ComposeSection.Stack
}

func (m *Manifest) HasDependenciesSection() bool {
	if m == nil {
		return false
	}
	return m.IsV2 && len(m.Dependencies) > 0
}

func (m *Manifest) HasBuildSection() bool {
	if m == nil {
		return false
	}
	return m.IsV2 && len(m.Build) > 0
}

func (m *Manifest) HasDeploySection() bool {
	if m == nil {
		return false
	}
	return m.IsV2 &&
		m.Deploy != nil &&
		(len(m.Deploy.Commands) > 0 ||
			(m.Deploy.ComposeSection != nil &&
				m.Deploy.ComposeSection.ComposesInfo != nil))
}
