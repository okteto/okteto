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
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compose-spec/godotenv"
	"github.com/google/uuid"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var (
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
	Probes               *Probes               `json:"probes,omitempty" yaml:"probes,omitempty"`
	NodeSelector         map[string]string     `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Metadata             *Metadata             `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Affinity             *Affinity             `json:"affinity,omitempty" yaml:"affinity,omitempty"`
	Image                string                `json:"image,omitempty" yaml:"image,omitempty"`
	Lifecycle            *Lifecycle            `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
	Replicas             *int                  `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	InitContainer        InitContainer         `json:"initContainer,omitempty" yaml:"initContainer,omitempty"`
	Workdir              string                `json:"workdir,omitempty" yaml:"workdir,omitempty"`
	Name                 string                `json:"name,omitempty" yaml:"name,omitempty"`
	Container            string                `json:"container,omitempty" yaml:"container,omitempty"`
	ServiceAccount       string                `json:"serviceAccount,omitempty" yaml:"serviceAccount,omitempty"`
	PriorityClassName    string                `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
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

	Autocreate bool `json:"autocreate,omitempty" yaml:"autocreate,omitempty"`
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
	LocalPath  string `json:"localPath,omitempty" yaml:"localPath,omitempty"`
	RemotePath string `json:"remotePath,omitempty" yaml:"remotePath,omitempty"`
}

// ExternalVolume represents a external volume in the development container
type ExternalVolume struct {
	Name      string
	SubPath   string
	MountPath string
}

// PersistentVolumeInfo info about the persistent volume
type PersistentVolumeInfo struct {
	Annotations  Annotations                      `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Labels       Labels                           `json:"labels,omitempty" yaml:"labels,omitempty"`
	AccessMode   apiv1.PersistentVolumeAccessMode `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	Size         string                           `json:"size,omitempty" yaml:"size,omitempty"`
	StorageClass string                           `json:"storageClass,omitempty" yaml:"storageClass,omitempty"`
	VolumeMode   apiv1.PersistentVolumeMode       `json:"volumeMode,omitempty" yaml:"volumeMode,omitempty"`
	Enabled      bool                             `json:"enabled,omitempty" yaml:"enabled"`
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
	ReadOnlyRootFilesystem   *bool         `json:"readOnlyRootFilesystem,omitempty" yaml:"readOnlyRootFilesystem,omitempty"`
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
	PostStart *LifecycleHandler `json:"postStart,omitempty" yaml:"postStart,omitempty"`
	PreStop   *LifecycleHandler `json:"preStop,omitempty" yaml:"preStop,omitempty"`
}

// LifecycleHandler defines a handler for lifecycle events
type LifecycleHandler struct {
	Command Command `json:"command,omitempty" yaml:"command,omitempty"`
	Enabled bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// ResourceList is a set of (resource name, quantity) pairs.
type ResourceList map[apiv1.ResourceName]resource.Quantity

// Labels is a set of (key, value) pairs.
type Labels map[string]string

// Selector is a set of (key, value) pairs.
type Selector map[string]string

// Annotations is a set of (key, value) pairs.
type Annotations map[string]string

func NewDev() *Dev {
	return &Dev{
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
		InitContainer:        InitContainer{Image: config.NewImageConfig(oktetoLog.GetOutputWriter()).GetBinImage()},
		Metadata: &Metadata{
			Labels:      Labels{},
			Annotations: Annotations{},
		},
	}
}

// loadAbsPaths makes every path used in the dev struct an absolute paths
func (dev *Dev) loadAbsPaths(devPath string, fs afero.Fs) error {
	devDir, err := filepath.Abs(filepath.Dir(devPath))
	if err != nil {
		return err
	}

	dev.loadVolumeAbsPaths(devDir, fs)
	for _, s := range dev.Services {
		s.loadVolumeAbsPaths(devDir, fs)
	}
	return nil
}

func (dev *Dev) loadVolumeAbsPaths(folder string, fs afero.Fs) {
	for i := range dev.Volumes {
		if dev.Volumes[i].LocalPath == "" {
			continue
		}
		dev.Volumes[i].LocalPath = loadAbsPath(folder, dev.Volumes[i].LocalPath, fs)
	}
	for i := range dev.Sync.Folders {
		dev.Sync.Folders[i].LocalPath = loadAbsPath(folder, dev.Sync.Folders[i].LocalPath, fs)
	}
}

func loadAbsPath(folder, path string, fs afero.Fs) string {
	if filepath.IsAbs(path) {
		realpath, err := filesystem.Realpath(fs, path)
		if err != nil {
			oktetoLog.Infof("error getting real path of %s: %s", path, err.Error())
			return path
		}
		return realpath
	}

	path = filepath.Join(folder, path)
	realpath, err := filesystem.Realpath(fs, path)
	if err != nil {
		oktetoLog.Infof("error getting real path of %s: %s", path, err.Error())
		return path
	}
	return realpath
}

func (dev *Dev) expandEnvVars() error {
	if err := dev.loadName(); err != nil {
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
	if dev.Image != "" {
		dev.Image, err = env.ExpandEnvIfNotEmpty(dev.Image)
		if err != nil {
			return err
		}
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
		dev.InitContainer.Image = config.NewImageConfig(oktetoLog.GetOutputWriter()).GetBinImage()
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
		dev.SecurityContext.RunAsUser = ptr.To(int64(0))
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
func (dev *Dev) PreparePathsAndExpandEnvFiles(manifestPath string, fs afero.Fs) error {
	if err := dev.loadAbsPaths(manifestPath, fs); err != nil {
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

// SetLastBuiltAnnotation sets the dev timestamp
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

// TranslatePodAffinity translates the affinity of pod to be all on the same node
func TranslatePodAffinity(tr *TranslationRule, name string) {
	if tr.Affinity == nil {
		tr.Affinity = &apiv1.Affinity{}
	}
	if tr.Affinity.PodAffinity == nil {
		tr.Affinity.PodAffinity = &apiv1.PodAffinity{}
	}
	if tr.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		tr.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = []apiv1.PodAffinityTerm{}
	}
	tr.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
		tr.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
		apiv1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					InteractiveDevLabel: name,
				},
			},
			TopologyKey: "kubernetes.io/hostname",
		},
	)
}

// ToTranslationRule translates a dev struct into a translation rule
func (dev *Dev) ToTranslationRule(main *Dev, namespace, username string, reset bool) *TranslationRule {
	rule := &TranslationRule{
		Container:         dev.Container,
		ImagePullPolicy:   dev.ImagePullPolicy,
		Environment:       dev.Environment,
		Secrets:           dev.Secrets,
		WorkDir:           dev.Workdir,
		PersistentVolume:  main.PersistentVolumeEnabled(),
		Volumes:           []VolumeMount{},
		SecurityContext:   dev.SecurityContext,
		ServiceAccount:    dev.ServiceAccount,
		PriorityClassName: main.PriorityClassName,
		Resources:         dev.Resources,
		InitContainer:     dev.InitContainer,
		Probes:            dev.Probes,
		Lifecycle:         dev.Lifecycle,
		NodeSelector:      dev.NodeSelector,
		Affinity:          (*apiv1.Affinity)(dev.Affinity),
	}

	if dev.IsHybridModeEnabled() {
		rule.WorkDir = "/okteto"
	}

	if dev.Image != "" {
		rule.Image = dev.Image
	}

	if rule.Healthchecks {
		rule.Probes = &Probes{Liveness: true, Startup: true, Readiness: true}
	}

	if areProbesEnabled(rule.Probes) {
		rule.Healthchecks = true
	}
	if main == dev {
		rule.Marker = config.NewImageConfig(oktetoLog.GetOutputWriter()).GetBinImage() // for backward compatibility
		rule.OktetoBinImageTag = dev.InitContainer.Image
		rule.Environment = append(
			rule.Environment,
			env.Var{
				Name:  "OKTETO_NAMESPACE",
				Value: namespace,
			},
			env.Var{
				Name:  "OKTETO_NAME",
				Value: dev.Name,
			},
		)

		if username != "" {
			rule.Environment = append(
				rule.Environment,
				env.Var{
					Name:  "OKTETO_USERNAME",
					Value: username,
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
			VolumeMount{
				Name:      main.GetVolumeName(),
				MountPath: "/var/syncthing/data",
				SubPath:   "syncthing-data",
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
	} else {
		if main.PersistentVolumeAccessMode() != apiv1.ReadWriteMany {
			// force all pods to land in the same node
			TranslatePodAffinity(rule, main.Name)
		}
		if len(dev.Args.Values) > 0 {
			rule.Args = dev.Args.Values
		} else if len(dev.Command.Values) > 0 {
			rule.Command = dev.Command.Values
			rule.Args = []string{}
		}
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

// HasEmptyResources returns true if the dev resources are empty (no resources specified in okteto manifest)
func (r *ResourceRequirements) HasEmptyResources() bool {
	return len(r.Requests) == 0 && len(r.Limits) == 0
}

// HasEmptyNodeSelector checks if the dev configuration has an empty nodeSelector
func (dev *Dev) HasEmptyNodeSelector() bool {
	return len(dev.NodeSelector) == 0
}

// GetTimeout returns the timeout override
func GetTimeout() (time.Duration, error) {
	defaultTimeout := 60 * time.Second

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

func (service *Dev) validateForExtraFields() error {
	errorMessage := "%q is not supported in Services. Please visit https://www.okteto.com/docs/reference/okteto-manifest/#services-object-optional for documentation"
	if service.Autocreate {
		return fmt.Errorf(errorMessage, "autocreate")
	}
	if service.Secrets != nil {
		return fmt.Errorf(errorMessage, "secrets")
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
	if service.PriorityClassName != "" {
		return fmt.Errorf(errorMessage, "priorityClassName")
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
