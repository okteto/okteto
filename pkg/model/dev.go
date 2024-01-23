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
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compose-spec/godotenv"
	"github.com/google/uuid"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/forward"
	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
)

var (
	// OktetoBinImageTag image tag with okteto internal binaries
	OktetoBinImageTag = "okteto/bin:1.4.4"

	errBadName = fmt.Errorf("Invalid name: must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character")

	// ValidKubeNameRegex is the regex to validate a kubernetes resource name
	ValidKubeNameRegex = regexp.MustCompile(`[^a-z0-9\-]+`)
)

// Dev represents a development container
type Dev struct {
	Resources            ResourceRequirements  `json:"resources,omitempty" yaml:"resources,omitempty"`
	Selector             Selector              `json:"selector,omitempty" yaml:"selector,omitempty"`
	PersistentVolumeInfo *PersistentVolumeInfo `json:"persistentVolume,omitempty" yaml:"persistentVolume,omitempty"`
	SecurityContext      *SecurityContext      `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
	Annotations          Annotations           `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Labels               Labels                `json:"labels,omitempty" yaml:"labels,omitempty"` // Deprecated field
	Probes               *Probes               `json:"probes,omitempty" yaml:"probes,omitempty"`
	NodeSelector         map[string]string     `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Metadata             *Metadata             `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Affinity             *Affinity             `json:"affinity,omitempty" yaml:"affinity,omitempty"`
	Image                *build.Info           `json:"image,omitempty" yaml:"image,omitempty"`
	Push                 *build.Info           `json:"-" yaml:"push,omitempty"`
	Lifecycle            *Lifecycle            `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
	Replicas             *int                  `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	InitContainer        InitContainer         `json:"initContainer,omitempty" yaml:"initContainer,omitempty"`
	Workdir              string                `json:"workdir,omitempty" yaml:"workdir,omitempty"`
	Name                 string                `json:"name,omitempty" yaml:"name,omitempty"`
	Username             string                `json:"-" yaml:"-"`
	RegistryURL          string                `json:"-" yaml:"-"`
	Context              string                `json:"context,omitempty" yaml:"context,omitempty"`
	Namespace            string                `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Container            string                `json:"container,omitempty" yaml:"container,omitempty"`
	ServiceAccount       string                `json:"serviceAccount,omitempty" yaml:"serviceAccount,omitempty"`
	parentSyncFolder     string
	Interface            string           `json:"interface,omitempty" yaml:"interface,omitempty"`
	Mode                 string           `json:"mode,omitempty" yaml:"mode,omitempty"`
	ImagePullPolicy      apiv1.PullPolicy `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`

	Tolerations     []apiv1.Toleration `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
	Command         Command            `json:"command,omitempty" yaml:"command,omitempty"`
	Forward         []forward.Forward  `json:"forward,omitempty" yaml:"forward,omitempty"`
	Reverse         []Reverse          `json:"reverse,omitempty" yaml:"reverse,omitempty"`
	ExternalVolumes []ExternalVolume   `json:"externalVolumes,omitempty" yaml:"externalVolumes,omitempty"`
	Secrets         []Secret           `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Volumes         []Volume           `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	EnvFiles        env.Files          `json:"envFiles,omitempty" yaml:"envFiles,omitempty"`
	Environment     env.Environment    `json:"environment,omitempty" yaml:"environment,omitempty"`
	Services        []*Dev             `json:"services,omitempty" yaml:"services,omitempty"`
	Args            Command            `json:"args,omitempty" yaml:"args,omitempty"`
	Sync            Sync               `json:"sync,omitempty" yaml:"sync,omitempty"`
	Timeout         Timeout            `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	RemotePort      int                `json:"remote,omitempty" yaml:"remote,omitempty"`
	SSHServerPort   int                `json:"sshServerPort,omitempty" yaml:"sshServerPort,omitempty"`

	EmptyImage    bool `json:"-" yaml:"-"`
	InitFromImage bool `json:"initFromImage,omitempty" yaml:"initFromImage,omitempty"`
	Autocreate    bool `json:"autocreate,omitempty" yaml:"autocreate,omitempty"`
	Healthchecks  bool `json:"healthchecks,omitempty" yaml:"healthchecks,omitempty"` // Deprecated field
}

type Affinity apiv1.Affinity

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
	LocalPath      string       `json:"-" yaml:"-"`
	RemotePath     string       `json:"-" yaml:"-"`
	Folders        []SyncFolder `json:"folders,omitempty" yaml:"folders,omitempty"`
	RescanInterval int          `json:"rescanInterval,omitempty" yaml:"rescanInterval,omitempty"`
	Compression    bool         `json:"compression" yaml:"compression"`
	Verbose        bool         `json:"verbose" yaml:"verbose"`
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
	StorageClass string `json:"storageClass,omitempty" yaml:"storageClass,omitempty"`
	Size         string `json:"size,omitempty" yaml:"size,omitempty"`
	Enabled      bool   `json:"enabled,omitempty" yaml:"enabled"`
}

// InitContainer represents the initial container
type InitContainer struct {
	Resources ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	Image     string               `json:"image,omitempty" yaml:"image,omitempty"`
}

// Timeout represents the timeout for the command
type Timeout struct {
	Default   time.Duration `json:"default,omitempty" yaml:"default,omitempty"`
	Resources time.Duration `json:"resources,omitempty" yaml:"resources,omitempty"`
}

type Metadata struct {
	Labels      Labels      `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations Annotations `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

// Duration represents a duration
type Duration time.Duration

// SecurityContext represents a pod security context
type SecurityContext struct {
	RunAsUser                *int64        `json:"runAsUser,omitempty" yaml:"runAsUser,omitempty"`
	RunAsGroup               *int64        `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	FSGroup                  *int64        `json:"fsGroup,omitempty" yaml:"fsGroup,omitempty"`
	Capabilities             *Capabilities `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	RunAsNonRoot             *bool         `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	AllowPrivilegeEscalation *bool         `json:"allowPrivilegeEscalation,omitempty" yaml:"allowPrivilegeEscalation,omitempty"`
}

// Capabilities sets the linux capabilities of a container
type Capabilities struct {
	Add  []apiv1.Capability `json:"add,omitempty" yaml:"add,omitempty"`
	Drop []apiv1.Capability `json:"drop,omitempty" yaml:"drop,omitempty"`
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

// Lifecycle defines the lifecycle for containers
type Lifecycle struct {
	PostStart bool `json:"postStart,omitempty" yaml:"postStart,omitempty"`
	PostStop  bool `json:"postStop,omitempty" yaml:"postStop,omitempty"`
}

// ResourceList is a set of (resource name, quantity) pairs.
type ResourceList map[apiv1.ResourceName]resource.Quantity

// Labels is a set of (key, value) pairs.
type Labels map[string]string

// Selector is a set of (key, value) pairs.
type Selector map[string]string

// Annotations is a set of (key, value) pairs.
type Annotations map[string]string

// Get returns a Dev object from a given file
func Get(devPath string) (*Manifest, error) {
	b, err := os.ReadFile(devPath)
	if err != nil {
		return nil, err
	}

	manifest, err := Read(b)
	if err != nil {
		return nil, err
	}

	for _, dev := range manifest.Dev {
		if err := dev.translateDeprecatedVolumeFields(); err != nil {
			return nil, err
		}

		if err := dev.PreparePathsAndExpandEnvFiles(devPath); err != nil {
			return nil, err
		}
	}

	return manifest, nil
}
func NewDev() *Dev {
	return &Dev{
		Image:       &build.Info{},
		Push:        &build.Info{},
		Environment: make(env.Environment, 0),
		Secrets:     make([]Secret, 0),
		Forward:     make([]forward.Forward, 0),
		Volumes:     make([]Volume, 0),
		Sync: Sync{
			Folders: make([]SyncFolder, 0),
		},
		Services:             make([]*Dev, 0),
		PersistentVolumeInfo: &PersistentVolumeInfo{Enabled: true},
		Probes:               &Probes{},
		Lifecycle:            &Lifecycle{},
		InitContainer:        InitContainer{Image: OktetoBinImageTag},
		Metadata: &Metadata{
			Labels:      Labels{},
			Annotations: Annotations{},
		},
	}
}

// loadAbsPaths makes every path used in the dev struct an absolute paths
func (dev *Dev) loadAbsPaths(devPath string) error {
	devDir, err := filepath.Abs(filepath.Dir(devPath))
	if err != nil {
		return err
	}

	if dev.Image != nil {
		if uri, err := url.ParseRequestURI(dev.Image.Context); err != nil || (uri != nil && (uri.Scheme == "" || uri.Host == "")) {
			dev.Image.Context = loadAbsPath(devDir, dev.Image.Context)
			dev.Image.Dockerfile = loadAbsPath(devDir, dev.Image.Dockerfile)
		}
	}

	if dev.Push != nil {
		if uri, err := url.ParseRequestURI(dev.Push.Context); err != nil || (uri != nil && (uri.Scheme == "" || uri.Host == "")) {
			dev.Push.Context = loadAbsPath(devDir, dev.Push.Context)
			dev.Push.Dockerfile = loadAbsPath(devDir, dev.Push.Dockerfile)
		}
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
	if err := dev.loadSelector(); err != nil {
		return err
	}

	return dev.loadImage()
}

func (dev *Dev) loadName() error {
	var err error
	if len(dev.Name) > 0 {
		dev.Name, err = env.ExpandEnv(dev.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dev *Dev) loadNamespace() error {
	var err error
	if len(dev.Namespace) > 0 {
		dev.Namespace, err = env.ExpandEnv(dev.Namespace)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dev *Dev) loadContext() error {
	var err error
	if len(dev.Context) > 0 {
		dev.Context, err = env.ExpandEnv(dev.Context)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dev *Dev) loadSelector() error {
	var err error
	for i := range dev.Selector {
		dev.Selector[i], err = env.ExpandEnv(dev.Selector[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (dev *Dev) loadImage() error {
	var err error
	if dev.Image == nil {
		dev.Image = &build.Info{}
	}
	if len(dev.Image.Name) > 0 {
		dev.Image.Name, err = env.ExpandEnvIfNotEmpty(dev.Image.Name)
		if err != nil {
			return err
		}
	}
	if dev.Image.Name == "" {
		dev.EmptyImage = true
	}
	return nil
}

func (dev *Dev) IsHybridModeEnabled() bool {
	return dev.Mode == constants.OktetoHybridModeFieldValue
}

func (dev *Dev) SetDefaults() error {
	if dev.Command.Values == nil {
		dev.Command.Values = []string{"sh"}
	}
	if len(dev.Forward) > 0 {
		sort.SliceStable(dev.Forward, func(i, j int) bool {
			return dev.Forward[i].Less(&dev.Forward[j])
		})
	}
	if dev.Image == nil {
		dev.Image = &build.Info{}
	}
	dev.Image.SetBuildDefaults()
	if dev.Push == nil {
		dev.Push = &build.Info{}
	}
	dev.Push.SetBuildDefaults()

	if err := dev.setTimeout(); err != nil {
		return err
	}

	if dev.ImagePullPolicy == "" {
		dev.ImagePullPolicy = apiv1.PullAlways
	}
	if dev.Metadata == nil {
		dev.Metadata = &Metadata{}
	}
	if dev.Metadata.Annotations == nil {
		dev.Metadata.Annotations = make(Annotations)
	}
	if dev.Metadata.Labels == nil {
		dev.Metadata.Labels = make(Labels)
	}
	if dev.Selector == nil {
		dev.Selector = make(Selector)
	}

	if dev.InitContainer.Image == "" {
		dev.InitContainer.Image = OktetoBinImageTag
	}
	if dev.Healthchecks {
		oktetoLog.Yellow("The use of 'healthchecks' field is deprecated and will be removed in a future version. Please use the field 'probes' instead.")
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
		dev.Interface = Localhost
	}
	if dev.SSHServerPort == 0 {
		dev.SSHServerPort = oktetoDefaultSSHServerPort
	}

	dev.setRunAsUserDefaults(dev)

	if os.Getenv(OktetoRescanIntervalEnvVar) != "" {
		rescanInterval, err := strconv.Atoi(os.Getenv(OktetoRescanIntervalEnvVar))
		if err != nil {
			return fmt.Errorf("cannot parse 'OKTETO_RESCAN_INTERVAL' into an integer: %w", err)
		}
		dev.Sync.RescanInterval = rescanInterval
	} else if dev.Sync.RescanInterval == 0 {
		dev.Sync.RescanInterval = DefaultSyncthingRescanInterval
	}

	for _, s := range dev.Services {
		if s.ImagePullPolicy == "" {
			s.ImagePullPolicy = apiv1.PullAlways
		}
		if s.Metadata == nil {
			s.Metadata = &Metadata{
				Annotations: make(Annotations),
				Labels:      make(Labels),
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
			s.Annotations = Annotations{}
		}
		if s.Name != "" && len(s.Selector) > 0 {
			return fmt.Errorf("'name' and 'selector' cannot be defined at the same time for service '%s'", s.Name)
		}
		s.Namespace = ""
		s.Context = ""
		s.setRunAsUserDefaults(dev)
		s.Forward = make([]forward.Forward, 0)
		s.Reverse = make([]Reverse, 0)
		s.Secrets = make([]Secret, 0)
		s.Services = make([]*Dev, 0)
		s.Sync.Compression = false
		s.Sync.RescanInterval = DefaultSyncthingRescanInterval
		if s.Probes == nil {
			s.Probes = &Probes{}
		}
		if s.Lifecycle == nil {
			s.Lifecycle = &Lifecycle{}
		}
	}

	if dev.Mode == "" {
		dev.Mode = constants.OktetoSyncModeFieldValue
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
		dev.SecurityContext.RunAsUser = pointer.Int64(0)
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

// expandEnvFiles reads each env file and append all the variables to the environment
func (dev *Dev) expandEnvFiles() error {
	for _, envFile := range dev.EnvFiles {
		filename, err := env.ExpandEnv(envFile)
		if err != nil {
			return err
		}

		f, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer func() {
			if err := f.Close(); err != nil {
				oktetoLog.Debugf("Error closing file %s: %s", filename, err)
			}
		}()

		envMap, err := godotenv.ParseWithLookup(f, os.LookupEnv)
		if err != nil {
			return fmt.Errorf("error parsing env_file %s: %w", filename, err)
		}

		for _, e := range dev.Environment {
			delete(envMap, e.Name)
		}

		for name, value := range envMap {
			if value == "" {
				value = os.Getenv(name)
			}
			if value != "" {
				dev.Environment = append(
					dev.Environment,
					env.Var{Name: name, Value: value},
				)
			}
		}
	}

	return nil
}

// Validate validates if a dev environment is correctly formed
func (dev *Dev) Validate() error {
	if dev.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if dev.Image == nil {
		dev.Image = &build.Info{}
	}

	if dev.Replicas != nil {
		return fmt.Errorf("replicas cannot be specified for main dev container")
	}

	if ValidKubeNameRegex.MatchString(dev.Name) {
		return errBadName
	}

	if strings.HasPrefix(dev.Name, "-") || strings.HasSuffix(dev.Name, "-") {
		return errBadName
	}

	if err := validatePullPolicy(dev.ImagePullPolicy); err != nil {
		return err
	}

	if err := validateSecrets(dev.Secrets); err != nil {
		return err
	}
	if err := dev.validateSecurityContext(); err != nil {
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

	if err := dev.validateSync(); err != nil {
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

// PreparePathsAndExpandEnvFiles calls other methods required to have the dev ready to use
func (dev *Dev) PreparePathsAndExpandEnvFiles(manifestPath string) error {
	if err := dev.loadAbsPaths(manifestPath); err != nil {
		return err
	}

	if err := dev.expandEnvFiles(); err != nil {
		return err
	}

	dev.computeParentSyncFolder()

	return nil
}

func (dev *Dev) validateSync() error {
	for _, folder := range dev.Sync.Folders {
		validPath, err := os.Stat(folder.LocalPath)

		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return oktetoErrors.UserError{
					E:    fmt.Errorf("path '%s' does not exist", folder.LocalPath),
					Hint: "Update the 'sync' field in your okteto manifest file to a valid directory path",
				}
			}

			return oktetoErrors.UserError{
				E:    fmt.Errorf("File paths are not supported on sync fields"),
				Hint: "Update the 'sync' field in your okteto manifest file to a valid directory path",
			}
		}

		if !validPath.IsDir() {
			return oktetoErrors.UserError{
				E:    fmt.Errorf("File paths are not supported on sync fields"),
				Hint: "Update the 'sync' field in your okteto manifest file to a valid directory path",
			}
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
		if err := s.validate(); err != nil {
			return err
		}

		if _, ok := seen[s.GetFileName()]; ok {
			return fmt.Errorf("secrets with the same basename '%s' are not supported", s.GetFileName())
		}
		seen[s.GetFileName()] = true
	}
	return nil
}

// RunAsNonRoot returns true if the development container must run as a non-root user
func (dev *Dev) RunAsNonRoot() bool {
	if dev.SecurityContext == nil {
		return false
	}
	if dev.SecurityContext.RunAsNonRoot == nil {
		return false
	}
	return *dev.SecurityContext.RunAsNonRoot
}

// isRootUser returns true if a root user is specified
func (dev *Dev) isRootUser() bool {
	if dev.SecurityContext == nil {
		return false
	}
	if dev.SecurityContext.RunAsUser == nil {
		return false
	}
	return *dev.SecurityContext.RunAsUser == 0
}

// validateSecurityContext checks to see if a root user is specified with runAsNonRoot enabled
func (dev *Dev) validateSecurityContext() error {
	if dev.isRootUser() && dev.RunAsNonRoot() {
		return fmt.Errorf("Running as the root user breaks runAsNonRoot constraint of the securityContext")
	}
	return nil
}

// LoadRemote configures remote execution
func (dev *Dev) LoadRemote(pubKeyPath string) {
	if dev.RemotePort == 0 {
		p, err := GetAvailablePort(dev.Interface)
		if err != nil {
			oktetoLog.Infof("failed to get random port for SSH connection: %s", err)
			p = oktetoDefaultSSHServerPort
		}

		dev.RemotePort = p
		oktetoLog.Infof("remote port not set, using %d", dev.RemotePort)
	}

	p := Secret{
		LocalPath:  pubKeyPath,
		RemotePath: authorizedKeysPath,
		Mode:       0644,
	}

	oktetoLog.Infof("enabled remote mode")

	for i := range dev.Secrets {
		if dev.Secrets[i].LocalPath == p.LocalPath {
			return
		}
	}

	dev.Secrets = append(dev.Secrets, p)
}

// LoadForcePull force the dev pods to be recreated and pull the latest version of their image
func (dev *Dev) LoadForcePull() {
	restartUUID := uuid.New().String()
	dev.ImagePullPolicy = apiv1.PullAlways
	dev.Metadata.Annotations[OktetoRestartAnnotation] = restartUUID
	for _, s := range dev.Services {
		s.ImagePullPolicy = apiv1.PullAlways
		s.Metadata.Annotations[OktetoRestartAnnotation] = restartUUID
	}
	oktetoLog.Infof("enabled force pull")
}

// Save saves the okteto manifest in a given path
func (dev *Dev) Save(path string) error {
	marshalled, err := yaml.Marshal(dev)
	if err != nil {
		oktetoLog.Infof("failed to marshall development container: %s", err)
		return fmt.Errorf("Failed to generate your manifest")
	}

	if err := os.WriteFile(path, marshalled, 0600); err != nil {
		oktetoLog.Infof("failed to write okteto manifest at %s: %s", path, err)
		return fmt.Errorf("Failed to write your manifest")
	}

	return nil
}

// SerializeEnvironmentVars returns environment variables as a list of strings
func SerializeEnvironmentVars(envs env.Environment) []string {
	result := []string{}
	for _, e := range envs {
		result = append(result, e.String())
	}
	// // stable serialization
	sort.Strings(result)
	return result
}

// SetLastBuiltAnnotation sets the dev timestacmp
func (dev *Dev) SetLastBuiltAnnotation() {
	if dev.Metadata.Annotations == nil {
		dev.Metadata.Annotations = Annotations{}
	}
	dev.Metadata.Annotations[LastBuiltAnnotation] = time.Now().UTC().Format(constants.TimeFormat)
}

// GetVolumeName returns the okteto volume name for a given development container
func (dev *Dev) GetVolumeName() string {
	return fmt.Sprintf(OktetoVolumeNameTemplate, dev.Name)
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

// ToTranslationRule translates a dev struct into a translation rule
func (dev *Dev) ToTranslationRule(main *Dev, reset bool) *TranslationRule {
	rule := &TranslationRule{
		Container:        dev.Container,
		ImagePullPolicy:  dev.ImagePullPolicy,
		Environment:      dev.Environment,
		Secrets:          dev.Secrets,
		WorkDir:          dev.Workdir,
		PersistentVolume: main.PersistentVolumeEnabled(),
		Volumes:          []VolumeMount{},
		SecurityContext:  dev.SecurityContext,
		ServiceAccount:   dev.ServiceAccount,
		Resources:        dev.Resources,
		Healthchecks:     dev.Healthchecks,
		InitContainer:    dev.InitContainer,
		Probes:           dev.Probes,
		Lifecycle:        dev.Lifecycle,
		NodeSelector:     dev.NodeSelector,
		Affinity:         (*apiv1.Affinity)(dev.Affinity),
	}

	if dev.IsHybridModeEnabled() {
		rule.WorkDir = "/okteto"
	}

	if !dev.EmptyImage {
		rule.Image = dev.Image.Name
	}

	if rule.Healthchecks {
		rule.Probes = &Probes{Liveness: true, Startup: true, Readiness: true}
	}

	if areProbesEnabled(rule.Probes) {
		rule.Healthchecks = true
	}
	if main == dev {
		rule.Marker = OktetoBinImageTag // for backward compatibility
		rule.OktetoBinImageTag = dev.InitContainer.Image
		rule.Environment = append(
			rule.Environment,
			env.Var{
				Name:  "OKTETO_NAMESPACE",
				Value: dev.Namespace,
			},
			env.Var{
				Name:  "OKTETO_NAME",
				Value: dev.Name,
			},
		)
		if dev.Username != "" {
			rule.Environment = append(
				rule.Environment,
				env.Var{
					Name:  "OKTETO_USERNAME",
					Value: dev.Username,
				},
			)
		}

		// We want to minimize environment mutations, so only reconfigure the SSH
		// server port if a non-default is specified.
		if dev.SSHServerPort != oktetoDefaultSSHServerPort {
			rule.Environment = append(
				rule.Environment,
				env.Var{
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
		if reset {
			rule.Args = append(rule.Args, "-e")
		}
		if dev.Sync.Verbose {
			rule.Args = append(rule.Args, "-v")
		}
		for _, s := range rule.Secrets {
			filename := s.GetFileName()
			if strings.Contains(filename, ".stignore") {
				filename = filepath.Base(s.LocalPath)
			}
			rule.Args = append(rule.Args, "-s", fmt.Sprintf("%s:%s", filename, s.RemotePath))
		}
	} else if len(dev.Args.Values) > 0 {
		rule.Args = dev.Args.Values
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
		enableHistoryVolume(rule, main)
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

func enableHistoryVolume(rule *TranslationRule, main *Dev) {
	rule.Volumes = append(rule.Volumes,
		VolumeMount{
			Name:      main.GetVolumeName(),
			MountPath: "/var/okteto/bashrc",
			SubPath:   "okteto-bash-history",
		})

	rule.Environment = append(rule.Environment,
		env.Var{
			Name:  "HISTSIZE",
			Value: "10000000",
		},
		env.Var{
			Name:  "HISTFILESIZE",
			Value: "10000000",
		},
		env.Var{
			Name:  "HISTCONTROL",
			Value: "ignoreboth:erasedups",
		},
		env.Var{
			Name:  "HISTFILE",
			Value: "/var/okteto/bashrc/.bash_history",
		},
		env.Var{
			Name:  "BASHOPTS",
			Value: "histappend",
		},
		env.Var{
			Name:  "PROMPT_COMMAND",
			Value: "history -a ; history -c ; history -r",
		})
}

func areProbesEnabled(probes *Probes) bool {
	if probes != nil {
		return probes.Liveness || probes.Readiness || probes.Startup
	}
	return false
}

func areAllProbesEnabled(probes *Probes) bool {
	if probes != nil {
		return probes.Liveness && probes.Readiness && probes.Startup
	}
	return false
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

	if v, ok := os.LookupEnv(OktetoExecuteSSHEnvVar); ok && v == "false" {
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

// GetTimeout returns the timeout override
func GetTimeout() (time.Duration, error) {
	defaultTimeout := (60 * time.Second)

	t := os.Getenv(OktetoTimeoutEnvVar)
	if t == "" {
		return defaultTimeout, nil
	}

	parsed, err := time.ParseDuration(t)
	if err != nil {
		return 0, fmt.Errorf("OKTETO_TIMEOUT is not a valid duration: %s", t)
	}

	return parsed, nil
}

func (dev *Dev) translateDeprecatedMetadataFields() {
	if len(dev.Labels) > 0 {
		oktetoLog.Warning("The field 'labels' is deprecated and will be removed in a future version. Use the field 'selector' instead (https://okteto.com/docs/reference/manifest/#selector)")
		for k, v := range dev.Labels {
			dev.Selector[k] = v
		}
	}

	if len(dev.Annotations) > 0 {
		oktetoLog.Warning("The field 'annotations' is deprecated and will be removed in a future version. Use the field 'metadata.annotations' instead (https://okteto.com/docs/reference/manifest/#metadata)")
		for k, v := range dev.Annotations {
			dev.Metadata.Annotations[k] = v
		}
	}
	for indx, s := range dev.Services {
		if len(s.Labels) > 0 {
			oktetoLog.Warning("The field '%s.labels' is deprecated and will be removed in a future version. Use the field 'selector' instead (https://okteto.com/docs/reference/manifest/#selector)", s.Name)
			for k, v := range s.Labels {
				dev.Services[indx].Selector[k] = v
			}
		}

		if len(s.Annotations) > 0 {
			oktetoLog.Warning("The field 'annotations' is deprecated and will be removed in a future version. Use the field '%s.metadata.annotations' instead (https://okteto.com/docs/reference/manifest/#metadata)", s.Name)
			for k, v := range s.Annotations {
				dev.Services[indx].Metadata.Annotations[k] = v
			}
		}
	}
}

func (service *Dev) validateForExtraFields() error {
	errorMessage := "%q is not supported in Services. Please visit https://www.okteto.com/docs/reference/manifest/#services-object-optional for documentation"
	if service.Username != "" {
		return fmt.Errorf(errorMessage, "username")
	}
	if service.RegistryURL != "" {
		return fmt.Errorf(errorMessage, "registryURL")
	}
	if service.Autocreate {
		return fmt.Errorf(errorMessage, "autocreate")
	}
	if service.Context != "" {
		return fmt.Errorf(errorMessage, "context")
	}
	if service.Push != nil {
		return fmt.Errorf(errorMessage, "push")
	}
	if service.Secrets != nil {
		return fmt.Errorf(errorMessage, "secrets")
	}
	if service.Healthchecks {
		return fmt.Errorf(errorMessage, "healthchecks")
	}
	if service.Probes != nil {
		return fmt.Errorf(errorMessage, "probes")
	}
	if service.Lifecycle != nil {
		return fmt.Errorf(errorMessage, "lifecycle")
	}
	if service.SecurityContext != nil {
		return fmt.Errorf(errorMessage, "securityContext")
	}
	if service.ServiceAccount != "" {
		return fmt.Errorf(errorMessage, "serviceAccount")
	}
	if service.RemotePort != 0 {
		return fmt.Errorf(errorMessage, "remote")
	}
	if service.SSHServerPort != 0 {
		return fmt.Errorf(errorMessage, "sshServerPort")
	}
	if service.ExternalVolumes != nil {
		return fmt.Errorf(errorMessage, "externalVolumes")
	}
	if service.parentSyncFolder != "" {
		return fmt.Errorf(errorMessage, "parentSyncFolder")
	}
	if service.Forward != nil {
		return fmt.Errorf(errorMessage, "forward")
	}
	if service.Reverse != nil {
		return fmt.Errorf(errorMessage, "reverse")
	}
	if service.Interface != "" {
		return fmt.Errorf(errorMessage, "interface")
	}
	if service.Services != nil {
		return fmt.Errorf(errorMessage, "services")
	}
	if service.PersistentVolumeInfo != nil {
		return fmt.Errorf(errorMessage, "persistentVolume")
	}
	if service.InitContainer.Image != "" {
		return fmt.Errorf(errorMessage, "initContainer")
	}
	if service.InitFromImage {
		return fmt.Errorf(errorMessage, "initFromImage")
	}
	if service.Timeout != (Timeout{}) {
		return fmt.Errorf(errorMessage, "timeout")
	}
	return nil
}

// DevCloneName returns the name of the mirrored version of a given resource
func DevCloneName(name string) string {
	return fmt.Sprintf("%s-okteto", name)
}

func (dev *Dev) IsInteractive() bool {
	if len(dev.Command.Values) == 0 {
		return true
	}
	if len(dev.Command.Values) == 1 {
		switch dev.Command.Values[0] {
		case "sh", "bash":
			return true
		default:
			return false
		}
	}
	return false
}
