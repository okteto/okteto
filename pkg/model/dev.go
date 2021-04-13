// Copyright 2020 The Okteto Authors
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
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/a8m/envsubst"
	"github.com/google/uuid"
	"github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/log"
	yaml "gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	//Localhost localhost
	Localhost                   = "localhost"
	oktetoSSHServerPortVariable = "OKTETO_REMOTE_PORT"
	oktetoDefaultSSHServerPort  = 2222
	//OktetoDefaultPVSize default volume size
	OktetoDefaultPVSize = "2Gi"
	//OktetoUpCmd up command
	OktetoUpCmd = "up"
	//OktetoPushCmd push command
	OktetoPushCmd = "push"

	//DeprecatedOktetoVolumeName name of the (deprecated) okteto persistent volume
	DeprecatedOktetoVolumeName = "okteto"
	//OktetoVolumeNameTemplate name template of the development container persistent volume
	OktetoVolumeNameTemplate = "okteto-%s"
	//DataSubPath subpath in the development container persistent volume for the data volumes
	DataSubPath = "data"
	//SourceCodeSubPath subpath in the development container persistent volume for the source code
	SourceCodeSubPath = "src"
	//OktetoSyncthingMountPath syncthing volume mount path
	OktetoSyncthingMountPath = "/var/syncthing"
	//RemoteMountPath remote volume mount path
	RemoteMountPath = "/var/okteto/remote"
	//SyncthingSubPath subpath in the development container persistent volume for the syncthing data
	SyncthingSubPath = "syncthing"
	//DefaultSyncthingRescanInterval default syncthing re-scan interval
	DefaultSyncthingRescanInterval = 300
	//RemoteSubPath subpath in the development container persistent volume for the remote data
	RemoteSubPath = "okteto-remote"
	//OktetoAutoCreateAnnotation indicates if the deployment was auto generatted by okteto up
	OktetoAutoCreateAnnotation = "dev.okteto.com/auto-create"
	//OktetoRestartAnnotation indicates the dev pod must be recreated to pull the latest version of its image
	OktetoRestartAnnotation = "dev.okteto.com/restart"
	//OktetoStignoreAnnotation indicates the hash of the stignore files to force redeployment
	OktetoStignoreAnnotation = "dev.okteto.com/stignore"

	//OktetoInitContainer name of the okteto init container
	OktetoInitContainer = "okteto-init"

	//DefaultImage default image for sandboxes
	DefaultImage = "okteto/dev:latest"

	//TranslationVersion version of the translation schema
	TranslationVersion = "1.0"

	//ResourceAMDGPU amd.com/gpu resource
	ResourceAMDGPU apiv1.ResourceName = "amd.com/gpu"
	//ResourceNVIDIAGPU nvidia.com/gpu resource
	ResourceNVIDIAGPU apiv1.ResourceName = "nvidia.com/gpu"

	// this path is expected by remote
	authorizedKeysPath = "/var/okteto/remote/authorized_keys"

	syncFieldDocsURL = "https://okteto.com/docs/reference/manifest#sync-string-required"
)

var (
	//OktetoBinImageTag image tag with okteto internal binaries
	OktetoBinImageTag = "okteto/bin:1.2.25"

	errBadName = fmt.Errorf("Invalid name: must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character")

	// ValidKubeNameRegex is the regex to validate a kubernetes resource name
	ValidKubeNameRegex = regexp.MustCompile(`[^a-z0-9\-]+`)

	rootUser int64

	// DevReplicas is the number of dev replicas
	DevReplicas int32 = 1

	devTerminationGracePeriodSeconds int64

	once sync.Once
)

//Dev represents a development container
type Dev struct {
	Name                 string                `json:"name" yaml:"name"`
	Autocreate           bool                  `json:"autocreate,omitempty" yaml:"autocreate,omitempty"`
	Labels               map[string]string     `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations          map[string]string     `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Tolerations          []apiv1.Toleration    `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
	Context              string                `json:"context,omitempty" yaml:"context,omitempty"`
	Namespace            string                `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Container            string                `json:"container,omitempty" yaml:"container,omitempty"`
	EmptyImage           bool                  `json:"-" yaml:"-"`
	Image                *BuildInfo            `json:"image,omitempty" yaml:"image,omitempty"`
	Push                 *BuildInfo            `json:"-" yaml:"push,omitempty"`
	ImagePullPolicy      apiv1.PullPolicy      `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	Environment          []EnvVar              `json:"environment,omitempty" yaml:"environment,omitempty"`
	Secrets              []Secret              `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Command              Command               `json:"command,omitempty" yaml:"command,omitempty"`
	Healthchecks         bool                  `json:"healthchecks,omitempty" yaml:"healthchecks,omitempty"`
	Probes               *Probes               `json:"probes,omitempty" yaml:"probes,omitempty"`
	WorkDir              string                `json:"workdir,omitempty" yaml:"workdir,omitempty"`
	MountPath            string                `json:"mountpath,omitempty" yaml:"mountpath,omitempty"`
	SubPath              string                `json:"subpath,omitempty" yaml:"subpath,omitempty"`
	SecurityContext      *SecurityContext      `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
	ServiceAccount       string                `json:"serviceAccount,omitempty" yaml:"serviceAccount,omitempty"`
	RemotePort           int                   `json:"remote,omitempty" yaml:"remote,omitempty"`
	SSHServerPort        int                   `json:"sshServerPort,omitempty" yaml:"sshServerPort,omitempty"`
	Volumes              []Volume              `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	ExternalVolumes      []ExternalVolume      `json:"externalVolumes,omitempty" yaml:"externalVolumes,omitempty"`
	Sync                 Sync                  `json:"sync,omitempty" yaml:"sync,omitempty"`
	parentSyncFolder     string                `json:"-" yaml:"-"`
	Forward              []Forward             `json:"forward,omitempty" yaml:"forward,omitempty"`
	Reverse              []Reverse             `json:"reverse,omitempty" yaml:"reverse,omitempty"`
	Interface            string                `json:"interface,omitempty" yaml:"interface,omitempty"`
	Resources            ResourceRequirements  `json:"resources,omitempty" yaml:"resources,omitempty"`
	Services             []*Dev                `json:"services,omitempty" yaml:"services,omitempty"`
	PersistentVolumeInfo *PersistentVolumeInfo `json:"persistentVolume,omitempty" yaml:"persistentVolume,omitempty"`
	InitContainer        InitContainer         `json:"initContainer,omitempty" yaml:"initContainer,omitempty"`
	Timeout              time.Duration         `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

//Entrypoint represents the start command of a development container
type Entrypoint struct {
	Values []string
}

//Command represents the start command of a development container
type Command struct {
	Values []string
}

//Args represents the args of a development container
type Args struct {
	Values []string
}

// BuildInfo represents the build info to generate an image
type BuildInfo struct {
	Name       string   `yaml:"name,omitempty"`
	Context    string   `yaml:"context,omitempty"`
	Dockerfile string   `yaml:"dockerfile,omitempty"`
	CacheFrom  []string `yaml:"cache_from,omitempty"`
	Target     string   `yaml:"target,omitempty"`
	Args       []EnvVar `yaml:"args,omitempty"`
}

// Volume represents a volume in the development container
type Volume struct {
	LocalPath  string
	RemotePath string
}

// Sync represents a sync info in the development container
type Sync struct {
	Compression    bool         `json:"compression" yaml:"compression"`
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

// SecurityContext represents a pod security context
type SecurityContext struct {
	RunAsUser    *int64        `json:"runAsUser,omitempty" yaml:"runAsUser,omitempty"`
	RunAsGroup   *int64        `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	FSGroup      *int64        `json:"fsGroup,omitempty" yaml:"fsGroup,omitempty"`
	Capabilities *Capabilities `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
}

// Capabilities sets the linux capabilities of a container
type Capabilities struct {
	Add  []apiv1.Capability `json:"add,omitempty" yaml:"add,omitempty"`
	Drop []apiv1.Capability `json:"drop,omitempty" yaml:"drop,omitempty"`
}

// EnvVar represents an environment value. When loaded, it will expand from the current env
type EnvVar struct {
	Name  string `yaml:"name,omitempty"`
	Value string `yaml:"value,omitempty"`
}

// Secret represents a development secret
type Secret struct {
	LocalPath  string
	RemotePath string
	Mode       int32
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

// ResourceList is a set of (resource name, quantity) pairs.
type ResourceList map[apiv1.ResourceName]resource.Quantity

//Get returns a Dev object from a given file
func Get(devPath string) (*Dev, error) {
	b, err := ioutil.ReadFile(devPath)
	if err != nil {
		return nil, err
	}

	dev, err := Read(b)
	if err != nil {
		return nil, err
	}

	if err := dev.translateDeprecatedVolumeFields(); err != nil {
		return nil, err
	}

	if err := dev.loadAbsPaths(devPath); err != nil {
		return nil, err
	}

	if err := dev.validate(); err != nil {
		return nil, err
	}

	dev.computeParentSyncFolder()

	return dev, nil
}

//Read reads an okteto manifests
func Read(bytes []byte) (*Dev, error) {
	dev := &Dev{
		Image:       &BuildInfo{},
		Push:        &BuildInfo{},
		Environment: make([]EnvVar, 0),
		Secrets:     make([]Secret, 0),
		Forward:     make([]Forward, 0),
		Volumes:     make([]Volume, 0),
		Sync: Sync{
			Folders: make([]SyncFolder, 0),
		},
		Services:             make([]*Dev, 0),
		PersistentVolumeInfo: &PersistentVolumeInfo{Enabled: true},
		Probes:               &Probes{},
	}

	if bytes != nil {
		if err := yaml.UnmarshalStrict(bytes, dev); err != nil {
			if strings.HasPrefix(err.Error(), "yaml: unmarshal errors:") {
				var sb strings.Builder
				_, _ = sb.WriteString("Invalid manifest:\n")
				l := strings.Split(err.Error(), "\n")
				for i := 1; i < len(l); i++ {
					e := strings.TrimSuffix(l[i], "in type model.Dev")
					e = strings.TrimSpace(e)
					_, _ = sb.WriteString(fmt.Sprintf("    - %s\n", e))
				}

				_, _ = sb.WriteString("    See https://okteto.com/docs/reference/manifest for details")
				return nil, errors.New(sb.String())
			}

			msg := strings.Replace(err.Error(), "yaml: unmarshal errors:", "invalid manifest:", 1)
			msg = strings.TrimSuffix(msg, "in type model.Dev")
			return nil, errors.New(msg)
		}
	}

	if err := dev.expandEnvVars(); err != nil {
		return nil, err
	}
	for _, s := range dev.Services {
		if err := s.expandEnvVars(); err != nil {
			return nil, err
		}
	}

	if err := dev.setDefaults(); err != nil {
		return nil, err
	}

	sort.SliceStable(dev.Forward, func(i, j int) bool {
		return dev.Forward[i].less(&dev.Forward[j])
	})

	sort.SliceStable(dev.Reverse, func(i, j int) bool {
		return dev.Reverse[i].Local < dev.Reverse[j].Local
	})

	return dev, nil
}

func (dev *Dev) loadAbsPaths(devPath string) error {
	devDir, err := filepath.Abs(filepath.Dir(devPath))
	if err != nil {
		return err
	}
	dev.Image.Context = loadAbsPath(devDir, dev.Image.Context)
	dev.Image.Dockerfile = loadAbsPath(devDir, dev.Image.Dockerfile)
	dev.Push.Context = loadAbsPath(devDir, dev.Push.Context)
	dev.Push.Dockerfile = loadAbsPath(devDir, dev.Push.Dockerfile)
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
		dev.Volumes[i].LocalPath = loadAbsPath(folder, dev.Volumes[i].LocalPath)
	}
	for i := range dev.Sync.Folders {
		dev.Sync.Folders[i].LocalPath = loadAbsPath(folder, dev.Sync.Folders[i].LocalPath)
	}
}

func loadAbsPath(folder, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(folder, path)
}

func (dev *Dev) expandEnvVars() error {
	if err := dev.loadName(); err != nil {
		return err
	}
	if err := dev.loadNamespace(); err != nil {
		return err
	}
	if err := dev.loadContext(); err != nil {
		return err
	}
	if err := dev.loadLabels(); err != nil {
		return err
	}

	return dev.loadImage()
}

func (dev *Dev) loadName() error {
	var err error
	if len(dev.Name) > 0 {
		dev.Name, err = ExpandEnv(dev.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dev *Dev) loadNamespace() error {
	var err error
	if len(dev.Namespace) > 0 {
		dev.Namespace, err = ExpandEnv(dev.Namespace)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dev *Dev) loadContext() error {
	var err error
	if len(dev.Context) > 0 {
		dev.Context, err = ExpandEnv(dev.Context)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dev *Dev) loadLabels() error {
	var err error
	for i := range dev.Labels {
		dev.Labels[i], err = ExpandEnv(dev.Labels[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (dev *Dev) loadImage() error {
	var err error
	if dev.Image == nil {
		dev.Image = &BuildInfo{}
	}
	if len(dev.Image.Name) > 0 {
		dev.Image.Name, err = ExpandEnv(dev.Image.Name)
		if err != nil {
			return err
		}
	}
	if dev.Image.Name == "" {
		dev.EmptyImage = true
	}
	return nil
}

func (dev *Dev) setDefaults() error {
	if dev.Command.Values == nil {
		dev.Command.Values = []string{"sh"}
	}
	setBuildDefaults(dev.Image)
	setBuildDefaults(dev.Push)

	if err := dev.setTimeout(); err != nil {
		return err
	}

	if dev.ImagePullPolicy == "" {
		dev.ImagePullPolicy = apiv1.PullAlways
	}
	if dev.Labels == nil {
		dev.Labels = map[string]string{}
	}
	if dev.Annotations == nil {
		dev.Annotations = map[string]string{}
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
	if dev.Interface == "" {
		dev.Interface = Localhost
	}
	if dev.SSHServerPort == 0 {
		dev.SSHServerPort = oktetoDefaultSSHServerPort
	}
	dev.setRunAsUserDefaults(dev)

	if os.Getenv("OKTETO_RESCAN_INTERVAL") != "" {
		rescanInterval, err := strconv.Atoi(os.Getenv("OKTETO_RESCAN_INTERVAL"))
		if err != nil {
			return fmt.Errorf("cannot parse 'OKTETO_RESCAN_INTERVAL' into an integer: %s", err.Error())
		}
		dev.Sync.RescanInterval = rescanInterval
	} else if dev.Sync.RescanInterval == 0 {
		dev.Sync.RescanInterval = DefaultSyncthingRescanInterval
	}
	if dev.InitContainer.Image == "" {
		dev.InitContainer.Image = OktetoBinImageTag
	}

	for _, s := range dev.Services {
		if s.ImagePullPolicy == "" {
			s.ImagePullPolicy = apiv1.PullAlways
		}
		if s.Labels == nil {
			s.Labels = map[string]string{}
		}
		if s.Annotations == nil {
			s.Annotations = map[string]string{}
		}
		if s.Name != "" && len(s.Labels) > 0 {
			return fmt.Errorf("'name' and 'labels' cannot be defined at the same time for service '%s'", s.Name)
		}
		s.Namespace = ""
		s.Context = ""
		s.setRunAsUserDefaults(dev)
		s.Forward = make([]Forward, 0)
		s.Reverse = make([]Reverse, 0)
		s.Secrets = make([]Secret, 0)
		s.Services = make([]*Dev, 0)
		s.Sync.Compression = false
		s.Sync.RescanInterval = DefaultSyncthingRescanInterval
		if s.Probes == nil {
			s.Probes = &Probes{}
		}
	}

	return nil
}

func setBuildDefaults(build *BuildInfo) {
	if build.Context == "" {
		build.Context = "."
	}
	if build.Dockerfile == "" {
		build.Dockerfile = filepath.Join(build.Context, "Dockerfile")
	}
}

func (dev *Dev) setRunAsUserDefaults(main *Dev) {
	if !main.PersistentVolumeEnabled() {
		return
	}
	if dev.SecurityContext == nil {
		dev.SecurityContext = &SecurityContext{}
	}
	if dev.SecurityContext.RunAsUser == nil {
		dev.SecurityContext.RunAsUser = &rootUser
	}
	if dev.SecurityContext.RunAsGroup == nil {
		dev.SecurityContext.RunAsGroup = dev.SecurityContext.RunAsUser
	}
	if dev.SecurityContext.FSGroup == nil {
		dev.SecurityContext.FSGroup = dev.SecurityContext.RunAsUser
	}
}

func (dev *Dev) setTimeout() error {
	if dev.Timeout != 0 {
		return nil
	}

	t, err := GetTimeout()
	if err != nil {
		return err
	}

	dev.Timeout = t
	return nil
}

func (dev *Dev) validate() error {
	if dev.Name == "" {
		return fmt.Errorf("Name cannot be empty")
	}

	if ValidKubeNameRegex.MatchString(dev.Name) {
		return errBadName
	}

	if strings.HasPrefix(dev.Name, "-") || strings.HasSuffix(dev.Name, "-") {
		return errBadName
	}

	if dev.SubPath != "" {
		return fmt.Errorf("'subpath' is not supported in the main dev container")
	}

	if err := validatePullPolicy(dev.ImagePullPolicy); err != nil {
		return err
	}

	if err := validateSecrets(dev.Secrets); err != nil {
		return err
	}

	if err := dev.validatePersistentVolume(); err != nil {
		return err
	}

	if err := dev.validateVolumes(nil); err != nil {
		return err
	}

	if err := dev.validateExternalVolumes(); err != nil {
		return err
	}

	if _, err := resource.ParseQuantity(dev.PersistentVolumeSize()); err != nil {
		return fmt.Errorf("'persistentVolume.size' is not valid. A sample value would be '10Gi'")
	}

	if dev.SSHServerPort <= 0 {
		return fmt.Errorf("'sshServerPort' must be > 0")
	}

	for _, s := range dev.Services {
		if err := validatePullPolicy(s.ImagePullPolicy); err != nil {
			return err
		}
		if err := s.validateVolumes(dev); err != nil {
			return err
		}
	}

	return nil
}

func validatePullPolicy(pullPolicy apiv1.PullPolicy) error {
	switch pullPolicy {
	case apiv1.PullAlways:
	case apiv1.PullIfNotPresent:
	case apiv1.PullNever:
	default:
		return fmt.Errorf("supported values for 'imagePullPolicy' are: 'Always', 'IfNotPresent' or 'Never'")
	}
	return nil
}

func validateSecrets(secrets []Secret) error {
	seen := map[string]bool{}
	for _, s := range secrets {
		if _, ok := seen[s.GetFileName()]; ok {
			return fmt.Errorf("Secrets with the same basename '%s' are not supported", s.GetFileName())
		}
		seen[s.GetFileName()] = true
	}
	return nil
}

//LoadRemote configures remote execution
func (dev *Dev) LoadRemote(pubKeyPath string) {
	if dev.RemotePort == 0 {
		p, err := GetAvailablePort(dev.Interface)
		if err != nil {
			log.Infof("failed to get random port for SSH connection: %s", err)
			p = oktetoDefaultSSHServerPort
		}

		dev.RemotePort = p
		log.Infof("remote port not set, using %d", dev.RemotePort)
	}

	p := Secret{
		LocalPath:  pubKeyPath,
		RemotePath: authorizedKeysPath,
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
	dev.Annotations[OktetoRestartAnnotation] = restartUUID
	for _, s := range dev.Services {
		s.ImagePullPolicy = apiv1.PullAlways
		s.Annotations[OktetoRestartAnnotation] = restartUUID
	}
	log.Infof("enabled force pull")
}

//Save saves the okteto manifest in a given path
func (dev *Dev) Save(path string) error {
	marshalled, err := yaml.Marshal(dev)
	if err != nil {
		log.Infof("failed to marshall development container: %s", err)
		return fmt.Errorf("Failed to generate your manifest")
	}

	if err := ioutil.WriteFile(path, marshalled, 0600); err != nil {
		log.Infof("failed to write okteto manifest at %s: %s", path, err)
		return fmt.Errorf("Failed to write your manifest")
	}

	return nil
}

//SerializeBuildArgs returns build  aaargs as a llist of strings
func SerializeBuildArgs(buildArgs []EnvVar) []string {
	result := []string{}
	for _, e := range buildArgs {
		result = append(
			result,
			fmt.Sprintf("%s=%s", e.Name, e.Value),
		)
	}
	return result
}

//SetLastBuiltAnnotation sets the dev timestacmp
func (dev *Dev) SetLastBuiltAnnotation() {
	if dev.Annotations == nil {
		dev.Annotations = map[string]string{}
	}
	dev.Annotations[labels.LastBuiltAnnotation] = time.Now().UTC().Format(labels.TimeFormat)
}

//GetVolumeName returns the okteto volume name for a given development container
func (dev *Dev) GetVolumeName() string {
	return fmt.Sprintf(OktetoVolumeNameTemplate, dev.Name)
}

// LabelsSelector returns the labels of a Deployment as a k8s selector
func (dev *Dev) LabelsSelector() string {
	labels := ""
	for k := range dev.Labels {
		if labels == "" {
			labels = fmt.Sprintf("%s=%s", k, dev.Labels[k])
		} else {
			labels = fmt.Sprintf("%s, %s=%s", labels, k, dev.Labels[k])
		}
	}
	return labels
}

// ToTranslationRule translates a dev struct into a translation rule
func (dev *Dev) ToTranslationRule(main *Dev) *TranslationRule {
	rule := &TranslationRule{
		Container:        dev.Container,
		ImagePullPolicy:  dev.ImagePullPolicy,
		Environment:      dev.Environment,
		Secrets:          dev.Secrets,
		WorkDir:          dev.WorkDir,
		PersistentVolume: main.PersistentVolumeEnabled(),
		Volumes:          []VolumeMount{},
		SecurityContext:  dev.SecurityContext,
		ServiceAccount:   dev.ServiceAccount,
		Resources:        dev.Resources,
		Healthchecks:     dev.Healthchecks,
		InitContainer:    dev.InitContainer,
		Probes:           dev.Probes,
	}

	if !dev.EmptyImage {
		rule.Image = dev.Image.Name
	}

	if rule.Healthchecks {
		rule.Probes = &Probes{Liveness: true, Startup: true, Readiness: true}
	}

	if areHealthchecksEnabled(rule.Probes) {
		rule.Healthchecks = true
	}
	if main == dev {
		rule.Marker = OktetoBinImageTag //for backward compatibility
		rule.OktetoBinImageTag = OktetoBinImageTag
		rule.Environment = append(
			rule.Environment,
			EnvVar{
				Name:  "OKTETO_NAMESPACE",
				Value: dev.Namespace,
			},
			EnvVar{
				Name:  "OKTETO_NAME",
				Value: dev.Name,
			},
		)

		// We want to minimize environment mutations, so only reconfigure the SSH
		// server port if a non-default is specified.
		if dev.SSHServerPort != oktetoDefaultSSHServerPort {
			rule.Environment = append(
				rule.Environment,
				EnvVar{
					Name:  oktetoSSHServerPortVariable,
					Value: strconv.Itoa(dev.SSHServerPort),
				},
			)
		}
		rule.Volumes = append(
			rule.Volumes,
			VolumeMount{
				Name:      main.GetVolumeName(),
				MountPath: OktetoSyncthingMountPath,
				SubPath:   SyncthingSubPath,
			},
		)
		if main.RemoteModeEnabled() {
			rule.Volumes = append(
				rule.Volumes,
				VolumeMount{
					Name:      main.GetVolumeName(),
					MountPath: RemoteMountPath,
					SubPath:   RemoteSubPath,
				},
			)
		}
		rule.Command = []string{"/var/okteto/bin/start.sh"}
		if main.RemoteModeEnabled() {
			rule.Args = []string{"-r"}
		} else {
			rule.Args = []string{}
		}
		for _, s := range rule.Secrets {
			rule.Args = append(rule.Args, "-s", fmt.Sprintf("%s:%s", s.GetFileName(), s.RemotePath))
		}
		if !main.PersistentVolumeEnabled() {
			rule.Args = append(rule.Args, "-e")
		}
	} else if len(dev.Command.Values) > 0 {
		rule.Command = dev.Command.Values
		rule.Args = []string{}
	}

	if main.PersistentVolumeEnabled() {
		for _, v := range dev.Volumes {
			rule.Volumes = append(
				rule.Volumes,
				VolumeMount{
					Name:      main.GetVolumeName(),
					MountPath: v.RemotePath,
					SubPath:   getDataSubPath(v.RemotePath),
				},
			)
		}
		for _, sync := range dev.Sync.Folders {
			rule.Volumes = append(
				rule.Volumes,
				VolumeMount{
					Name:      main.GetVolumeName(),
					MountPath: sync.RemotePath,
					SubPath:   main.getSourceSubPath(sync.LocalPath),
				},
			)
		}
	}

	for _, v := range dev.ExternalVolumes {
		rule.Volumes = append(
			rule.Volumes,
			VolumeMount{
				Name:      v.Name,
				MountPath: v.MountPath,
				SubPath:   v.SubPath,
			},
		)
	}

	return rule
}

func areHealthchecksEnabled(probes *Probes) bool {
	if probes != nil {
		return probes.Liveness || probes.Readiness || probes.Startup
	}
	return false
}

func areAllHealthchecksEnabled(probes *Probes) bool {
	if probes != nil {
		return probes.Liveness && probes.Readiness && probes.Startup
	}
	return false
}

//GevSandbox returns a deployment sandbox
func (dev *Dev) GevSandbox() *appsv1.Deployment {
	image := dev.Image.Name
	if image == "" {
		image = DefaultImage
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dev.Name,
			Namespace: dev.Namespace,
			Annotations: map[string]string{
				OktetoAutoCreateAnnotation: OktetoUpCmd,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &DevReplicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": dev.Name,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": dev.Name,
					},
				},
				Spec: apiv1.PodSpec{
					ServiceAccountName:            dev.ServiceAccount,
					TerminationGracePeriodSeconds: &devTerminationGracePeriodSeconds,
					Containers: []apiv1.Container{
						{
							Name:            "dev",
							Image:           image,
							ImagePullPolicy: apiv1.PullAlways,
						},
					},
				},
			},
		},
	}
}

// RemoteModeEnabled returns true if remote is enabled
func (dev *Dev) RemoteModeEnabled() bool {
	if dev == nil {
		return true
	}

	if dev.RemotePort > 0 {
		return true
	}

	if len(dev.Reverse) > 0 {
		return true
	}

	if v, ok := os.LookupEnv("OKTETO_EXECUTE_SSH"); ok && v == "false" {
		return false
	}
	return true
}

// GetKeyName returns the secret key name
func (s *Secret) GetKeyName() string {
	return fmt.Sprintf("dev-secret-%s", filepath.Base(s.RemotePath))
}

// GetFileName returns the secret file name
func (s *Secret) GetFileName() string {
	return filepath.Base(s.RemotePath)
}

//ExpandEnv expands the environments supporting the notation "${var:-$DEFAULT}"
func ExpandEnv(value string) (string, error) {
	result, err := envsubst.String(value)
	if err != nil {
		return "", fmt.Errorf("error expanding environment on '%s': %s", value, err.Error())
	}
	return result, nil
}

// GetTimeout returns the timeout override
func GetTimeout() (time.Duration, error) {
	defaultTimeout := (30 * time.Second)

	t := os.Getenv("OKTETO_TIMEOUT")
	if t == "" {
		return defaultTimeout, nil
	}

	parsed, err := time.ParseDuration(t)
	if err != nil {
		return 0, fmt.Errorf("OKTETO_TIMEOUT is not a valid duration: %s", t)
	}

	return parsed, nil
}
