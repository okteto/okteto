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

	"github.com/a8m/envsubst"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deps"
	"github.com/okteto/okteto/pkg/discovery"
	"github.com/okteto/okteto/pkg/env"
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
	buildHeadComment = "The build section defines how to build the images of your development environment\nMore info: https://www.okteto.com/docs/reference/okteto-manifest/#build"
	buildExample     = `build:
  my-service:
    context: .`
	buildSvcEnvVars   = "You can use the following env vars to refer to this image in your deploy commands:\n - OKTETO_BUILD_%s_REGISTRY: image registry\n - OKTETO_BUILD_%s_REPOSITORY: image repo\n - OKTETO_BUILD_%s_IMAGE: image name\n - OKTETO_BUILD_%s_SHA: image tag sha256"
	deployHeadComment = "The deploy section defines how to deploy your development environment\nMore info: https://www.okteto.com/docs/reference/okteto-manifest/#deploy"
	deployExample     = `deploy:
  commands:
  - name: Deploy
    command: echo 'Replace this line with the proper 'helm' or 'kubectl' commands to deploy your development environment'`
	devHeadComment = "The dev section defines how to activate a development container\nMore info: https://www.okteto.com/docs/reference/okteto-manifest/#dev"
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
	dependenciesHeadComment = "The dependencies section defines other git repositories to be deployed as part of your development environment\nMore info: https://www.okteto.com/docs/reference/okteto-manifest/#dependencies"
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
	Fs           afero.Fs                 `json:"-" yaml:"-"`
	External     externalresource.Section `json:"external,omitempty" yaml:"external,omitempty"`
	Dependencies deps.ManifestSection     `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Build        build.ManifestBuild      `json:"build,omitempty" yaml:"build,omitempty"`
	Deploy       *DeployInfo              `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	Dev          ManifestDevs             `json:"dev,omitempty" yaml:"dev,omitempty"`
	Name         string                   `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace    string                   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Context      string                   `json:"context,omitempty" yaml:"context,omitempty"`
	Icon         string                   `json:"icon,omitempty" yaml:"icon,omitempty"`
	ManifestPath string                   `json:"-" yaml:"-"`
	Destroy      *DestroyInfo             `json:"destroy,omitempty" yaml:"destroy,omitempty"`
	Test         ManifestTests            `json:"test,omitempty" yaml:"test,omitempty"`

	Type          Archetype               `json:"-" yaml:"-"`
	GlobalForward []forward.GlobalForward `json:"forward,omitempty" yaml:"forward,omitempty"`
	Manifest      []byte                  `json:"-" yaml:"-"`
	IsV2          bool                    `json:"-" yaml:"-"`
}

// ManifestDevs defines all the dev section
type ManifestDevs map[string]*Dev

// ManifestTests defines all the test sections
type ManifestTests map[string]*Test

// ImageFromManifest is a thunk that returns an image value from a parsed manifest
// This allows to implement general purpose logic on images without necessarily
// referencing a specific image, for eg manifest.Deploy.Image or manifest.Destroy.Image
type ImageFromManifest func(manifest *Manifest) string

// NewManifest creates a new empty manifest
func NewManifest() *Manifest {
	return &Manifest{
		Dev:           map[string]*Dev{},
		Test:          map[string]*Test{},
		Build:         map[string]*build.Info{},
		Dependencies:  deps.ManifestSection{},
		Deploy:        &DeployInfo{},
		GlobalForward: []forward.GlobalForward{},
		External:      externalresource.Section{},
		Fs:            afero.NewOsFs(),
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
		Build: build.ManifestBuild{},
		IsV2:  true,
		Fs:    afero.NewOsFs(),
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
	name, err := env.ExpandEnv(dev.Name)
	if err != nil {
		oktetoLog.Infof("could not expand dev name '%s'", dev.Name)
		name = dev.Name
	}
	manifest.Dev[name] = dev
	return manifest
}

// DeployInfo represents what must be deployed for the app to work
type DeployInfo struct {
	ComposeSection *ComposeSectionInfo `json:"compose,omitempty" yaml:"compose,omitempty"`
	Endpoints      EndpointSpec        `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	Divert         *DivertDeploy       `json:"divert,omitempty" yaml:"divert,omitempty"`
	Image          string              `json:"image,omitempty" yaml:"image,omitempty"`
	Commands       []DeployCommand     `json:"commands,omitempty" yaml:"commands,omitempty"`
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
	DeprecatedDeployment string                 `json:"deployment,omitempty" yaml:"deployment,omitempty"`
	VirtualServices      []DivertVirtualService `json:"virtualServices,omitempty" yaml:"virtualServices,omitempty"`
	Hosts                []DivertHost           `json:"hosts,omitempty" yaml:"hosts,omitempty"`
	DeprecatedPort       int                    `json:"port,omitempty" yaml:"port,omitempty"`
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
	Stack        *Stack          `json:"-" yaml:"-"`
	ComposesInfo ComposeInfoList `json:"manifest,omitempty" yaml:"manifest,omitempty"`
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

func getManifestFromOktetoFile(cwd string, fs afero.Fs) (*Manifest, error) {
	manifestPath, err := discovery.GetOktetoManifestPath(cwd)
	if err != nil {
		return nil, err
	}
	oktetoLog.Infof("Found okteto manifest file on path: %s", manifestPath)
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Found okteto manifest on %s", manifestPath)
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Unmarshalling manifest...")
	devManifest, err := getManifestFromFile(cwd, manifestPath, fs)
	if err != nil {
		return nil, err
	}
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Okteto manifest unmarshalled successfully")
	return devManifest, nil
}

func getManifestFromDevFilePath(cwd, manifestPath string, fs afero.Fs) (*Manifest, error) {
	if manifestPath != "" && !filepath.IsAbs(manifestPath) {
		manifestPath = filepath.Join(cwd, manifestPath)
	}
	if manifestPath != "" && filesystem.FileExistsAndNotDir(manifestPath, afero.NewOsFs()) {
		return getManifestFromFile(cwd, manifestPath, fs)
	}

	return nil, discovery.ErrOktetoManifestNotFound
}

// GetManifestV1 gets a manifest from a path or search for the files to generate it
func GetManifestV1(manifestPath string, fs afero.Fs) (*Manifest, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	manifest, err := getManifestFromDevFilePath(cwd, manifestPath, fs)
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

	manifest, err = getManifestFromOktetoFile(cwd, fs)
	if err != nil {
		return nil, err
	}

	return manifest, nil
}

func pathExistsAndDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// GetManifestV2 gets a manifest from a path or search for the files to generate it
func GetManifestV2(manifestPath string, fs afero.Fs) (*Manifest, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	manifest, err := getManifestFromDevFilePath(cwd, manifestPath, fs)
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

	manifest, err = getManifestFromOktetoFile(cwd, fs)
	if err != nil {
		if !errors.Is(err, discovery.ErrOktetoManifestNotFound) {
			return nil, err
		}
	}

	if manifest != nil && manifest.IsV2 {
		return manifest, nil
	}

	inferredManifest, err := GetInferredManifest(cwd, fs)
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
func getManifestFromFile(cwd, manifestPath string, fs afero.Fs) (*Manifest, error) {
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
			Test:  ManifestTests{},
			Build: build.ManifestBuild{},
			IsV2:  true,
			Fs:    fs,
		}
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Unmarshalling compose...")

		var composeFiles []string
		for _, composeInfo := range stackManifest.Deploy.ComposeSection.ComposesInfo {
			composeFiles = append(composeFiles, composeInfo.File)
		}
		// We need to ensure that LoadStack has false because we don't want to expand env vars
		s, stackErr := LoadStack("", composeFiles, false, fs)
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
			s, err := LoadStack("", stackFiles, false, fs)
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
func GetInferredManifest(cwd string, fs afero.Fs) (*Manifest, error) {
	pipelinePath, err := discovery.GetOktetoPipelinePath(cwd)
	if err == nil {
		oktetoLog.Infof("Found pipeline on: %s", pipelinePath)
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Found okteto pipeline manifest on %s", pipelinePath)
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Unmarshalling pipeline manifest...")
		pipelineManifest, err := GetManifestV2(pipelinePath, fs)
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
			Test:  ManifestTests{},
			Build: build.ManifestBuild{},
			IsV2:  true,
			Fs:    fs,
		}
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Unmarshalling compose...")
		var stackFiles []string
		for _, composeInfo := range stackManifest.Deploy.ComposeSection.ComposesInfo {
			stackFiles = append(stackFiles, composeInfo.File)
		}
		s, err := LoadStack("", stackFiles, true, fs)
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
			Test:  ManifestTests{},
			Build: build.ManifestBuild{},
			Fs:    fs,
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
			Test:  ManifestTests{},
			Build: build.ManifestBuild{},
			Fs:    fs,
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

	for name, external := range manifest.External {
		external.SetDefaults(name)
	}

	manifest.ManifestPath = devPath

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
		if dev.Image != nil && (dev.Image.Context != "" || dev.Image.Dockerfile != "") && !hasShownWarning {
			hasShownWarning = true
			oktetoLog.Yellow(`The 'image' extended syntax is deprecated and will be removed in a future version. Define the images you want to build in the 'build' section of your manifest. More info at https://www.okteto.com/docs/reference/okteto-manifest/#build"`)
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
	if err := m.Build.Validate(); err != nil {
		return err
	}
	return m.validateDivert()
}

func (s *Secret) validate() error {
	if s.LocalPath == "" || s.RemotePath == "" {
		return fmt.Errorf("secrets must follow the syntax 'LOCAL_PATH:REMOTE_PATH:MODE'")
	}

	if exists := filesystem.FileExistsAndNotDir(s.LocalPath, afero.NewOsFs()); !exists {
		return fmt.Errorf("secret '%s' is not a regular file", s.LocalPath)
	}

	if !strings.HasPrefix(s.RemotePath, "/") {
		return fmt.Errorf("secret remote path '%s' must be an absolute path", s.RemotePath)
	}

	return nil
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
		m.Deploy.Divert.Namespace, err = env.ExpandEnvIfNotEmpty(m.Deploy.Divert.Namespace)
		if err != nil {
			return err
		}
		for i := range m.Deploy.Divert.Hosts {
			m.Deploy.Divert.Hosts[i].VirtualService, err = env.ExpandEnvIfNotEmpty(m.Deploy.Divert.Hosts[i].VirtualService)
			if err != nil {
				return err
			}
			m.Deploy.Divert.Hosts[i].Namespace, err = env.ExpandEnvIfNotEmpty(m.Deploy.Divert.Hosts[i].Namespace)
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
			return fmt.Errorf("error on dev '%s': %w", d.Name, err)
		}
		for _, s := range d.Services {
			if err := s.expandEnvVars(); err != nil {
				return fmt.Errorf("error on dev '%s': %w", d.Name, err)
			}
			if err := s.validateForExtraFields(); err != nil {
				return fmt.Errorf("error on dev '%s': %w", d.Name, err)
			}
		}

		if err := d.SetDefaults(); err != nil {
			return fmt.Errorf("error on dev '%s': %w", d.Name, err)
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
		if b == nil {
			continue
		}
		if b.Name != "" {
			b.Context = b.Name
			b.Name = ""
		}

		if !(b.Image != "" && len(b.VolumesToInclude) > 0 && b.Dockerfile == "") {
			b.SetBuildDefaults()
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
			s, err := LoadStack("", stackFiles, true, manifest.Fs)
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
				expandedTag, err := env.ExpandEnv(tag)
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
			manifest.Destroy.Image, err = env.ExpandEnv(manifest.Destroy.Image)
			if err != nil {
				return err
			}
		}
	}

	for devName, devInfo := range manifest.Dev {
		if _, ok := manifest.Build[devName]; ok && devInfo.Image == nil && devInfo.Autocreate {
			devInfo.Image = &build.Info{
				Name: fmt.Sprintf("${OKTETO_BUILD_%s_IMAGE}", strings.ToUpper(strings.ReplaceAll(devName, "-", "_"))),
			}
		}
		if devInfo.Image != nil {
			devInfo.Image.Name, err = env.ExpandEnvIfNotEmpty(devInfo.Image.Name)
			if err != nil {
				return err
			}
		}
	}

	for _, mf := range manifest.Test {
		if mf.expandEnvVars() != nil {
			return err
		}
	}

	return nil
}

// InferFromStack infers data, mainly dev services and build information from services defined in the stackfile
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

		// Only if the build section of the service is empty, the service specifies an image and there are
		// volume mounts we should create a custom Dockerfile including the volumes and generate the proper build section
		if svcInfo.Build == nil && svcInfo.Image != "" && len(svcInfo.VolumeMounts) > 0 {
			// This check is to prevent that we modify the build section of the manifest if that service is already
			// defined there. In that case, we should just skip this and don't generate the custom Dockerfile and
			// build info
			if _, ok := m.Build[svcName]; ok {
				continue
			}

			context, err := getBuildContextForComposeWithVolumeMounts(m)
			if err != nil {
				return nil, err
			}

			for idx, volume := range svcInfo.VolumeMounts {
				localPath := volume.LocalPath
				if filepath.IsAbs(localPath) {
					localPath, err = filepath.Rel(context, volume.LocalPath)
					if err != nil {
						localPath, err = filepath.Rel(cwd, volume.LocalPath)
						if err != nil {
							oktetoLog.Info("can not find svc[%s].build.volumes to include relative to svc[%s].build.context", svcName, svcName)
						}
					}
				}
				volume.LocalPath = localPath
				svcInfo.VolumeMounts[idx] = volume
			}

			buildInfo, err = build.CreateDockerfileWithVolumeMounts(context, svcInfo.Image, svcInfo.VolumeMounts, m.Fs)
			if err != nil {
				return nil, err
			}

			svcInfo.Build = buildInfo
		} else {
			if svcInfo.Image != "" {
				buildInfo.Image = svcInfo.Image
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
					d.Image = &build.Info{Name: v.Image}
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

// getBuildContextForComposeWithVolumeMounts This function is very specific for the scenario of compose with
// volume mounts where an image wrapping the image specified in the compose is built. This heuristic to calculate
// the build context is:
//   - If the manifestPath property is set, we should use the directory where the manifest is located.
//     We use GetWorkdirFromManifestPath because it takes care of the case of .okteto directory
//   - If there is any compose info, we should use the directory where the first compose is located.
//     We use GetWorkdirFromManifestPath because it takes care of the case of .okteto directory
//   - If none of the other condition is met, we just use the current directory
func getBuildContextForComposeWithVolumeMounts(m *Manifest) (string, error) {
	context, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}

	hasComposeInfo := m.Deploy != nil &&
		m.Deploy.ComposeSection != nil &&
		len(m.Deploy.ComposeSection.ComposesInfo) > 0

	if m.ManifestPath != "" {
		context = GetWorkdirFromManifestPath(m.ManifestPath)
	} else if hasComposeInfo {
		context = GetWorkdirFromManifestPath(m.Deploy.ComposeSection.ComposesInfo[0].File)
	}

	return context, nil
}
