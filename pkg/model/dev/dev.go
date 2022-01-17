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

package dev

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/compose-spec/godotenv"
	"github.com/google/uuid"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/build"
	"github.com/okteto/okteto/pkg/model/constants"
	"github.com/okteto/okteto/pkg/model/environment"
	"github.com/okteto/okteto/pkg/model/files"
	"github.com/okteto/okteto/pkg/model/metadata"
	"github.com/okteto/okteto/pkg/model/port"
	"github.com/okteto/okteto/pkg/model/secrets"
	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
)

// Dev represents a development container
type Dev struct {
	Name                 string               `json:"name" yaml:"name"`
	Username             string               `json:"-" yaml:"-"`
	RegistryURL          string               `json:"-" yaml:"-"`
	Selector             Selector             `json:"selector,omitempty" yaml:"selector,omitempty"`
	Annotations          metadata.Annotations `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Tolerations          []apiv1.Toleration   `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
	Context              string               `json:"context,omitempty" yaml:"context,omitempty"`
	Namespace            string               `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Container            string               `json:"container,omitempty" yaml:"container,omitempty"`
	EmptyImage           bool                 `json:"-" yaml:"-"`
	Image                *build.Build         `json:"image,omitempty" yaml:"image,omitempty"`
	Push                 *build.Build         `json:"-" yaml:"push,omitempty"`
	ImagePullPolicy      apiv1.PullPolicy     `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	Secrets              []secrets.Secret     `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Command              Command              `json:"command,omitempty" yaml:"command,omitempty"`
	Probes               *Probes              `json:"probes,omitempty" yaml:"probes,omitempty"`
	Lifecycle            *Lifecycle           `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
	Workdir              string               `json:"workdir,omitempty" yaml:"workdir,omitempty"`
	SecurityContext      *SecurityContext     `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
	ServiceAccount       string               `json:"serviceAccount,omitempty" yaml:"serviceAccount,omitempty"`
	RemotePort           int                  `json:"remote,omitempty" yaml:"remote,omitempty"`
	SSHServerPort        int                  `json:"sshServerPort,omitempty" yaml:"sshServerPort,omitempty"`
	ExternalVolumes      []ExternalVolume     `json:"externalVolumes,omitempty" yaml:"externalVolumes,omitempty"`
	Sync                 Sync                 `json:"sync,omitempty" yaml:"sync,omitempty"`
	parentSyncFolder     string
	Forward              []port.Forward          `json:"forward,omitempty" yaml:"forward,omitempty"`
	Reverse              []Reverse               `json:"reverse,omitempty" yaml:"reverse,omitempty"`
	Interface            string                  `json:"interface,omitempty" yaml:"interface,omitempty"`
	Resources            ResourceRequirements    `json:"resources,omitempty" yaml:"resources,omitempty"`
	Services             []*Dev                  `json:"services,omitempty" yaml:"services,omitempty"`
	PersistentVolumeInfo *PersistentVolumeInfo   `json:"persistentVolume,omitempty" yaml:"persistentVolume,omitempty"`
	InitContainer        InitContainer           `json:"initContainer,omitempty" yaml:"initContainer,omitempty"`
	InitFromImage        bool                    `json:"initFromImage,omitempty" yaml:"initFromImage,omitempty"`
	Timeout              Timeout                 `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Docker               DinDContainer           `json:"docker,omitempty" yaml:"docker,omitempty"`
	Divert               *Divert                 `json:"divert,omitempty" yaml:"divert,omitempty"`
	NodeSelector         map[string]string       `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Affinity             *Affinity               `json:"affinity,omitempty" yaml:"affinity,omitempty"`
	Metadata             *metadata.Metadata      `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Autocreate           bool                    `json:"autocreate,omitempty" yaml:"autocreate,omitempty"`
	EnvFiles             environment.EnvFiles    `json:"envFiles,omitempty" yaml:"envFiles,omitempty"`
	Environment          environment.Environment `json:"environment,omitempty" yaml:"environment,omitempty"`
	Volumes              []Volume                `json:"volumes,omitempty" yaml:"volumes,omitempty"`

	//Deprecated fields
	Healthchecks bool            `json:"healthchecks,omitempty" yaml:"healthchecks,omitempty"`
	Labels       metadata.Labels `json:"labels,omitempty" yaml:"labels,omitempty"`
}

//Affinity defines affinity rules
type Affinity apiv1.Affinity

// Selector is a set of (key, value) pairs.
type Selector map[string]string

// Entrypoint represents the start command of a development container
type Entrypoint struct {
	Values []string
}

// Command represents the start command of a development container
type Command struct {
	Values []string
}

// Args represents the args of a development container
type Args struct {
	Values []string
}

// Volume represents a volume in the development container
type Volume struct {
	LocalPath  string
	RemotePath string
}

// Sync represents a sync info in the development container
type Sync struct {
	Compression    bool         `json:"compression" yaml:"compression"`
	Verbose        bool         `json:"verbose" yaml:"verbose"`
	RescanInterval int          `json:"rescanInterval,omitempty" yaml:"rescanInterval,omitempty"`
	Folders        []SyncFolder `json:"folders,omitempty" yaml:"folders,omitempty"`
	LocalPath      string
	RemotePath     string
}

// SyncFolder represents a sync folder in the development container
type SyncFolder struct {
	LocalPath  string
	RemotePath string
}

// ExternalVolume represents a external volume in the development container
type ExternalVolume struct {
	Name      string
	SubPath   string
	MountPath string
}

// PersistentVolumeInfo info about the persistent volume
type PersistentVolumeInfo struct {
	Enabled      bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	StorageClass string `json:"storageClass,omitempty" yaml:"storageClass,omitempty"`
	Size         string `json:"size,omitempty" yaml:"size,omitempty"`
}

// InitContainer represents the initial container
type InitContainer struct {
	Image     string               `json:"image,omitempty" yaml:"image,omitempty"`
	Resources ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// DinDContainer represents the DinD container
type DinDContainer struct {
	Enabled   bool                 `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Image     string               `json:"image,omitempty" yaml:"image,omitempty"`
	Resources ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// Timeout represents the timeout for the command
type Timeout struct {
	Default   time.Duration `json:"default,omitempty" yaml:"default,omitempty"`
	Resources time.Duration `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// Duration represents a duration
type Duration time.Duration

// SecurityContext represents a pod security context
type SecurityContext struct {
	RunAsUser    *int64        `json:"runAsUser,omitempty" yaml:"runAsUser,omitempty"`
	RunAsGroup   *int64        `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	FSGroup      *int64        `json:"fsGroup,omitempty" yaml:"fsGroup,omitempty"`
	Capabilities *Capabilities `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	RunAsNonRoot *bool         `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
}

// Capabilities sets the linux capabilities of a container
type Capabilities struct {
	Add  []apiv1.Capability `json:"add,omitempty" yaml:"add,omitempty"`
	Drop []apiv1.Capability `json:"drop,omitempty" yaml:"drop,omitempty"`
}

// Reverse represents a remote forward port
type Reverse struct {
	Remote int
	Local  int
}

// ResourceRequirements describes the compute resource requirements.
type ResourceRequirements struct {
	Limits   ResourceList `json:"limits,omitempty" yaml:"limits,omitempty"`
	Requests ResourceList `json:"requests,omitempty" yaml:"requests,omitempty"`
}

// Probes defines probes for containers
type Probes struct {
	Liveness  bool `json:"liveness,omitempty" yaml:"liveness,omitempty"`
	Readiness bool `json:"readiness,omitempty" yaml:"readiness,omitempty"`
	Startup   bool `json:"startup,omitempty" yaml:"startup,omitempty"`
}

// Lifecycle defines the lifecycle for containers
type Lifecycle struct {
	PostStart bool `json:"postStart,omitempty" yaml:"postStart,omitempty"`
	PostStop  bool `json:"postStop,omitempty" yaml:"postStop,omitempty"`
}

// Divert defines how to divert a given service
type Divert struct {
	Ingress string `yaml:"ingress,omitempty"`
	Service string `yaml:"service,omitempty"`
	Port    int    `yaml:"port,omitempty"`
}

// ResourceList is a set of (resource name, quantity) pairs.
type ResourceList map[apiv1.ResourceName]resource.Quantity

//NewDev creates a default dev struct
func NewDev() *Dev {
	return &Dev{
		Image:       &build.Build{},
		Push:        &build.Build{},
		Environment: environment.Environment{},
		Secrets:     []secrets.Secret{},
		Forward:     []port.Forward{},
		Volumes:     []Volume{},
		Sync: Sync{
			Folders: []SyncFolder{},
		},
		Services:             []*Dev{},
		PersistentVolumeInfo: &PersistentVolumeInfo{Enabled: true},
		Probes:               &Probes{},
		Lifecycle:            &Lifecycle{},
		InitContainer:        InitContainer{Image: constants.OktetoBinImageTag},
	}
}

//Save saves the okteto manifest in a given path
func (dev *Dev) Save(path string) error {
	marshalled, err := yaml.Marshal(dev)
	if err != nil {
		log.Infof("failed to marshall development container: %s", err)
		return fmt.Errorf("Failed to generate your manifest")
	}

	if err := os.WriteFile(path, marshalled, 0600); err != nil {
		log.Infof("failed to write okteto manifest at %s: %s", path, err)
		return fmt.Errorf("Failed to write your manifest")
	}

	return nil
}

//LoadAbsPaths translates relative path into absolute path
func (dev *Dev) LoadAbsPaths(devPath string) error {
	devDir, err := filepath.Abs(filepath.Dir(devPath))
	if err != nil {
		return err
	}

	if uri, err := url.ParseRequestURI(dev.Image.Context); err != nil || (uri != nil && (uri.Scheme == "" || uri.Host == "")) {
		dev.Image.Context = files.LoadAbsPath(devDir, dev.Image.Context)
		dev.Image.Dockerfile = files.LoadAbsPath(devDir, dev.Image.Dockerfile)
	}
	if uri, err := url.ParseRequestURI(dev.Push.Context); err != nil || (uri != nil && (uri.Scheme == "" || uri.Host == "")) {
		dev.Push.Context = files.LoadAbsPath(devDir, dev.Push.Context)
		dev.Push.Dockerfile = files.LoadAbsPath(devDir, dev.Push.Dockerfile)
	}

	dev.loadVolumeAbsPaths(devDir)
	for _, s := range dev.Services {
		s.loadVolumeAbsPaths(devDir)
	}
	return nil
}

func (dev *Dev) loadVolumeAbsPaths(folder string) {
	for i := range dev.Volumes {
		if dev.Volumes[i].LocalPath == "" {
			continue
		}
		dev.Volumes[i].LocalPath = files.LoadAbsPath(folder, dev.Volumes[i].LocalPath)
	}
	for i := range dev.Sync.Folders {
		dev.Sync.Folders[i].LocalPath = files.LoadAbsPath(folder, dev.Sync.Folders[i].LocalPath)
	}
}

//ExpandEnvVars expands environment variables from some fields
func (dev *Dev) ExpandEnvVars() error {
	if err := dev.loadName(); err != nil {
		return err
	}
	if err := dev.loadNamespace(); err != nil {
		return err
	}
	if err := dev.loadContext(); err != nil {
		return err
	}
	if err := dev.loadSelector(); err != nil {
		return err
	}

	return dev.loadImage()
}

func (dev *Dev) loadName() error {
	var err error
	if len(dev.Name) > 0 {
		dev.Name, err = environment.ExpandEnv(dev.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dev *Dev) loadNamespace() error {
	var err error
	if len(dev.Namespace) > 0 {
		dev.Namespace, err = environment.ExpandEnv(dev.Namespace)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dev *Dev) loadContext() error {
	var err error
	if len(dev.Context) > 0 {
		dev.Context, err = environment.ExpandEnv(dev.Context)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dev *Dev) loadSelector() error {
	var err error
	for i := range dev.Selector {
		dev.Selector[i], err = environment.ExpandEnv(dev.Selector[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (dev *Dev) loadImage() error {
	var err error
	if dev.Image == nil {
		dev.Image = &build.Build{}
	}
	if len(dev.Image.Name) > 0 {
		dev.Image.Name, err = environment.ExpandEnv(dev.Image.Name)
		if err != nil {
			return err
		}
	}
	if dev.Image.Name == "" {
		dev.EmptyImage = true
	}
	return nil
}

//SetDefaults sets build default info
func (dev *Dev) SetDefaults() error {
	if dev.Command.Values == nil {
		dev.Command.Values = []string{"sh"}
	}
	dev.Image.SetDefaults()
	dev.Push.SetDefaults()

	if err := dev.setTimeout(); err != nil {
		return err
	}

	if dev.ImagePullPolicy == "" {
		dev.ImagePullPolicy = apiv1.PullAlways
	}
	if dev.Metadata == nil {
		dev.Metadata = &metadata.Metadata{}
	}
	if dev.Metadata.Annotations == nil {
		dev.Metadata.Annotations = make(metadata.Annotations)
	}
	if dev.Metadata.Labels == nil {
		dev.Metadata.Labels = make(metadata.Labels)
	}
	if dev.Selector == nil {
		dev.Selector = make(Selector)
	}

	if dev.Healthchecks {
		log.Yellow("The use of 'healthchecks' field is deprecated and will be removed in a future release. Please use the field 'probes' instead.")
		if dev.Probes == nil {
			dev.Probes = &Probes{Liveness: true, Readiness: true, Startup: true}
		}
	}
	if dev.Probes == nil {
		dev.Probes = &Probes{}
	}
	if dev.Lifecycle == nil {
		dev.Lifecycle = &Lifecycle{}
	}
	if dev.Interface == "" {
		dev.Interface = constants.Localhost
	}
	if dev.SSHServerPort == 0 {
		dev.SSHServerPort = constants.OktetoDefaultSSHServerPort
	}
	dev.setRunAsUserDefaults(dev)

	if os.Getenv(constants.OktetoRescanIntervalEnvVar) != "" {
		rescanInterval, err := strconv.Atoi(os.Getenv(constants.OktetoRescanIntervalEnvVar))
		if err != nil {
			return fmt.Errorf("cannot parse 'OKTETO_RESCAN_INTERVAL' into an integer: %s", err.Error())
		}
		dev.Sync.RescanInterval = rescanInterval
	} else if dev.Sync.RescanInterval == 0 {
		dev.Sync.RescanInterval = constants.DefaultSyncthingRescanInterval
	}

	if dev.Docker.Enabled && dev.Docker.Image == "" {
		dev.Docker.Image = constants.DefaultDinDImage
	}

	for _, s := range dev.Services {
		if s.ImagePullPolicy == "" {
			s.ImagePullPolicy = apiv1.PullAlways
		}
		if s.Metadata == nil {
			s.Metadata = &metadata.Metadata{
				Annotations: make(metadata.Annotations),
				Labels:      make(metadata.Labels),
			}
		}
		if s.Metadata.Annotations == nil {
			s.Metadata.Annotations = map[string]string{}
		}
		if s.Metadata.Labels == nil {
			s.Metadata.Labels = map[string]string{}
		}
		if s.Selector == nil {
			s.Selector = map[string]string{}
		}
		if s.Annotations == nil {
			s.Annotations = metadata.Annotations{}
		}
		if s.Name != "" && len(s.Selector) > 0 {
			return fmt.Errorf("'name' and 'selector' cannot be defined at the same time for service '%s'", s.Name)
		}
		s.Namespace = ""
		s.Context = ""
		s.setRunAsUserDefaults(dev)
		s.Forward = make([]port.Forward, 0)
		s.Reverse = make([]Reverse, 0)
		s.Secrets = make([]secrets.Secret, 0)
		s.Services = make([]*Dev, 0)
		s.Sync.Compression = false
		s.Sync.RescanInterval = constants.DefaultSyncthingRescanInterval
		if s.Probes == nil {
			s.Probes = &Probes{}
		}
		if s.Lifecycle == nil {
			s.Lifecycle = &Lifecycle{}
		}
	}

	return nil
}

func (dev *Dev) setRunAsUserDefaults(main *Dev) {
	if !main.PersistentVolumeEnabled() {
		return
	}
	if dev.RunAsNonRoot() {
		return
	}
	if dev.SecurityContext == nil {
		dev.SecurityContext = &SecurityContext{}
	}
	if dev.SecurityContext.RunAsUser == nil {
		dev.SecurityContext.RunAsUser = pointer.Int64Ptr(0)
	}
	if dev.SecurityContext.RunAsGroup == nil {
		dev.SecurityContext.RunAsGroup = dev.SecurityContext.RunAsUser
	}
	if dev.SecurityContext.FSGroup == nil {
		dev.SecurityContext.FSGroup = dev.SecurityContext.RunAsUser
	}
}

func (dev *Dev) setTimeout() error {
	if dev.Timeout.Resources == 0 {
		dev.Timeout.Resources = 120 * time.Second
	}
	if dev.Timeout.Default != 0 {
		return nil
	}

	t, err := GetTimeout()
	if err != nil {
		return err
	}

	dev.Timeout.Default = t
	return nil
}

//ExpandEnvFiles expands environment variables from env file fields
func (dev *Dev) ExpandEnvFiles() error {
	for _, envFile := range dev.EnvFiles {
		filename, err := environment.ExpandEnv(envFile)
		if err != nil {
			return err
		}

		f, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		envMap, err := godotenv.ParseWithLookup(f, os.LookupEnv)
		if err != nil {
			return fmt.Errorf("error parsing env_file %s: %s", filename, err.Error())
		}

		for _, e := range dev.Environment {
			delete(envMap, e.Name)
		}

		for name, value := range envMap {
			dev.Environment = append(
				dev.Environment,
				environment.EnvVar{Name: name, Value: value},
			)
		}
	}

	return nil
}

// LoadRemote configures remote execution
func (dev *Dev) LoadRemote(pubKeyPath string) {
	if dev.RemotePort == 0 {
		p, err := port.GetAvailablePort(dev.Interface)
		if err != nil {
			log.Infof("failed to get random port for SSH connection: %s", err)
			p = constants.OktetoDefaultSSHServerPort
		}

		dev.RemotePort = p
		log.Infof("remote port not set, using %d", dev.RemotePort)
	}

	p := secrets.Secret{
		LocalPath:  pubKeyPath,
		RemotePath: constants.AuthorizedKeysPath,
		Mode:       0644,
	}

	log.Infof("enabled remote mode")

	for i := range dev.Secrets {
		if dev.Secrets[i].LocalPath == p.LocalPath {
			return
		}
	}

	dev.Secrets = append(dev.Secrets, p)
}

//LoadForcePull force the dev pods to be recreated and pull the latest version of their image
func (dev *Dev) LoadForcePull() {
	restartUUID := uuid.New().String()
	dev.ImagePullPolicy = apiv1.PullAlways
	dev.Metadata.Annotations[constants.OktetoRestartAnnotation] = restartUUID
	for _, s := range dev.Services {
		s.ImagePullPolicy = apiv1.PullAlways
		s.Metadata.Annotations[constants.OktetoRestartAnnotation] = restartUUID
	}
	log.Infof("enabled force pull")
}

//SetLastBuiltAnnotation sets the dev timestacmp
func (dev *Dev) SetLastBuiltAnnotation() {
	if dev.Metadata.Annotations == nil {
		dev.Metadata.Annotations = metadata.Annotations{}
	}
	dev.Metadata.Annotations[constants.LastBuiltAnnotation] = time.Now().UTC().Format(constants.TimeFormat)
}

//GetVolumeName returns the okteto volume name for a given development container
func (dev *Dev) GetVolumeName() string {
	return fmt.Sprintf(constants.OktetoVolumeNameTemplate, dev.Name)
}

// LabelsSelector returns the labels of a Deployment as a k8s selector
func (dev *Dev) LabelsSelector() string {
	labels := ""
	for k := range dev.Selector {
		if labels == "" {
			labels = fmt.Sprintf("%s=%s", k, dev.Selector[k])
		} else {
			labels = fmt.Sprintf("%s, %s=%s", labels, k, dev.Selector[k])
		}
	}
	return labels
}

//TranslateDeprecatedMetadataFields translates deprecated labels and annotations into metadata
func (dev *Dev) TranslateDeprecatedMetadataFields() error {
	if len(dev.Labels) > 0 {
		log.Warning("The field 'labels' is deprecated. Use the field 'selector' instead (%s)", constants.ManifestSelectorDocsURL)
		for k, v := range dev.Labels {
			dev.Selector[k] = v
		}
	}

	if len(dev.Annotations) > 0 {
		log.Warning("The field 'annotations' is deprecated. Use the field 'metadata.annotations' instead (%s)", constants.ManifestMetadataDocsURL)
		for k, v := range dev.Annotations {
			dev.Metadata.Annotations[k] = v
		}
	}
	for _, s := range dev.Services {
		if len(s.Labels) > 0 {
			log.Warning("The field '%s.labels' is deprecated. Use the field 'selector' instead (%s)", s.Name, constants.ManifestSelectorDocsURL)
			for k, v := range s.Labels {
				s.Selector[k] = v
			}
		}

		if len(s.Annotations) > 0 {
			log.Warning("The field 'annotations' is deprecated. Use the field '%s.metadata.annotations' instead (%s)", s.Name, constants.ManifestMetadataDocsURL)
			for k, v := range s.Annotations {
				s.Metadata.Annotations[k] = v
			}
		}
	}
	return nil
}

// GetTimeout returns the timeout override
func GetTimeout() (time.Duration, error) {
	defaultTimeout := (60 * time.Second)

	t := os.Getenv(constants.OktetoTimeoutEnvVar)
	if t == "" {
		return defaultTimeout, nil
	}

	parsed, err := time.ParseDuration(t)
	if err != nil {
		return 0, fmt.Errorf("OKTETO_TIMEOUT is not a valid duration: %s", t)
	}

	return parsed, nil
}

// DivertName returns the name of the diverted version of a given resource
func DivertName(name, username string) string {
	return fmt.Sprintf("%s-%s", name, username)
}

// DevCloneName returns the name of the mirrored version of a given resource
func DevCloneName(name string) string {
	return fmt.Sprintf("%s-okteto", name)
}
