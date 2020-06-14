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
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/okteto/okteto/pkg/log"
	yaml "gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// OktetoMarkerPathVariable is the marker used for syncthing
	OktetoMarkerPathVariable = "OKTETO_MARKER_PATH"

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
	//SourceCodeSubPath subpath in the development container persistent volume for the source code
	SourceCodeSubPath = "src"
	//OktetoSyncthingMountPath syncthing volume mount path
	OktetoSyncthingMountPath = "/var/syncthing"
	//SyncthingSubPath subpath in the development container persistent volume for the syncthing data
	SyncthingSubPath = "syncthing"
	//OktetoAutoCreateAnnotation indicates if the deployment was auto generatted by okteto up
	OktetoAutoCreateAnnotation = "dev.okteto.com/auto-create"
	//OktetoRestartAnnotation indicates the dev pod must be recreated to pull the latest version of its image
	OktetoRestartAnnotation = "dev.okteto.com/restart"

	//OktetoInitContainer name of the okteto init container
	OktetoInitContainer = "okteto-init"

	//DefaultImage default image for sandboxes
	DefaultImage = "okteto/desk:latest"

	//TranslationVersion version of the translation schema
	TranslationVersion = "1.0"

	//ResourceAMDGPU amd.com/gpu resource
	ResourceAMDGPU apiv1.ResourceName = "amd.com/gpu"
	//ResourceNVIDIAGPU nvidia.com/gpu resource
	ResourceNVIDIAGPU apiv1.ResourceName = "nvidia.com/gpu"

	// this path is expected by remote
	authorizedKeysPath = "/var/okteto/remote/authorized_keys"
)

var (
	errBadName = fmt.Errorf("Invalid name: must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character")

	// ValidKubeNameRegex is the regex to validate a kubernetes resource name
	ValidKubeNameRegex = regexp.MustCompile(`[^a-z0-9\-]+`)

	rootUser int64 = 0
	// DevReplicas is the number of dev replicas
	DevReplicas int32 = 1

	devTerminationGracePeriodSeconds int64
)

//Dev represents a development container
type Dev struct {
	Name                 string                `json:"name" yaml:"name"`
	Labels               map[string]string     `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations          map[string]string     `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Tolerations          []apiv1.Toleration    `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`
	Namespace            string                `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Container            string                `json:"container,omitempty" yaml:"container,omitempty"`
	Image                string                `json:"image,omitempty" yaml:"image,omitempty"`
	Build                *BuildInfo            `json:"-" yaml:"build,omitempty"`
	Push                 *BuildInfo            `json:"-" yaml:"push,omitempty"`
	ImagePullPolicy      apiv1.PullPolicy      `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	Environment          []EnvVar              `json:"environment,omitempty" yaml:"environment,omitempty"`
	Secrets              []Secret              `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Command              Command               `json:"command,omitempty" yaml:"command,omitempty"`
	Healthchecks         bool                  `json:"healthchecks,omitempty" yaml:"healthchecks,omitempty"`
	WorkDir              string                `json:"workdir,omitempty" yaml:"workdir,omitempty"`
	MountPath            string                `json:"mountpath,omitempty" yaml:"mountpath,omitempty"`
	SubPath              string                `json:"subpath,omitempty" yaml:"subpath,omitempty"`
	Forward              []Forward             `json:"forward,omitempty" yaml:"forward,omitempty"`
	Reverse              []Reverse             `json:"reverse,omitempty" yaml:"reverse,omitempty"`
	Volumes              []Volume              `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	PersistentVolumeInfo *PersistentVolumeInfo `json:"persistentVolume,omitempty" yaml:"persistentVolume,omitempty"`
	ExternalVolumes      []ExternalVolume      `json:"externalVolumes,omitempty" yaml:"externalVolumes,omitempty"`
	RemotePort           int                   `json:"remote,omitempty" yaml:"remote,omitempty"`
	Resources            ResourceRequirements  `json:"resources,omitempty" yaml:"resources,omitempty"`
	DevPath              string                `json:"-" yaml:"-"`
	DevDir               string                `json:"-" yaml:"-"`
	Services             []*Dev                `json:"services,omitempty" yaml:"services,omitempty"`
	SecurityContext      *SecurityContext      `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
	SSHServerPort        int                   `json:"sshServerPort,omitempty" yaml:"sshServerPort,omitempty"`
}

//Command represents the staart commaand of a development contaianer
type Command struct {
	Values []string
}

// BuildInfo represents the build info to generate an image
type BuildInfo struct {
	BuildInfoRaw
}

// BuildInfoRaw represents the build info for serialization
type BuildInfoRaw struct {
	Context    string   `yaml:"context,omitempty"`
	Dockerfile string   `yaml:"dockerfile,omitempty"`
	Target     string   `yaml:"target,omitempty"`
	Args       []EnvVar `yaml:"args,omitempty"`
}

// Volume represents a volume in the development container
type Volume struct {
	SubPath   string
	MountPath string
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

	if err := dev.validate(); err != nil {
		return nil, err
	}

	dev.DevDir, err = filepath.Abs(filepath.Dir(devPath))
	if err != nil {
		return nil, err
	}
	dev.DevPath = filepath.Base(devPath)

	return dev, nil
}

//Read reads an okteto manifests
func Read(bytes []byte) (*Dev, error) {
	dev := &Dev{
		Build:       &BuildInfo{},
		Push:        &BuildInfo{},
		Environment: make([]EnvVar, 0),
		Secrets:     make([]Secret, 0),
		Forward:     make([]Forward, 0),
		Volumes:     make([]Volume, 0),
		Services:    make([]*Dev, 0),
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

	dev.loadName()
	for _, s := range dev.Services {
		s.loadName()
	}

	dev.loadImage()
	for _, s := range dev.Services {
		s.loadImage()
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

func (dev *Dev) loadName() {
	if len(dev.Name) > 0 {
		dev.Name = os.ExpandEnv(dev.Name)
	}
}

func (dev *Dev) loadImage() {
	if len(dev.Image) > 0 {
		dev.Image = os.ExpandEnv(dev.Image)
	}
}

func (dev *Dev) setDefaults() error {
	if dev.Command.Values == nil {
		dev.Command.Values = []string{"sh"}
	}
	setBuildDefaults(dev.Build)
	setBuildDefaults(dev.Push)
	if dev.MountPath == "" && dev.WorkDir == "" {
		dev.MountPath = "/okteto"
		dev.WorkDir = "/okteto"
	}
	if dev.ImagePullPolicy == "" {
		dev.ImagePullPolicy = apiv1.PullAlways
	}
	if dev.WorkDir != "" && dev.MountPath == "" {
		dev.MountPath = dev.WorkDir
	}
	if dev.Labels == nil {
		dev.Labels = map[string]string{}
	}
	if dev.Annotations == nil {
		dev.Annotations = map[string]string{}
	}
	if dev.SSHServerPort == 0 {
		dev.SSHServerPort = oktetoDefaultSSHServerPort
	}
	dev.setRunAsUserDefaults(dev)
	for _, s := range dev.Services {
		if s.MountPath == "" && s.WorkDir == "" {
			s.MountPath = "/okteto"
			s.WorkDir = "/okteto"
		}
		if s.ImagePullPolicy == "" {
			s.ImagePullPolicy = apiv1.PullAlways
		}
		if s.WorkDir != "" && s.MountPath == "" {
			s.MountPath = s.WorkDir
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
		s.setRunAsUserDefaults(dev)
		s.Forward = make([]Forward, 0)
		s.Reverse = make([]Reverse, 0)
		s.Secrets = make([]Secret, 0)
		s.Services = make([]*Dev, 0)
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

	if !dev.PersistentVolumeEnabled() {
		if len(dev.Services) > 0 {
			return fmt.Errorf("'persistentVolume.enabled' must be set to true to work with services")
		}
		if len(dev.Volumes) > 0 {
			return fmt.Errorf("'persistentVolume.enabled' must be set to true to use volumes")
		}
	}

	if err := validateVolumes(dev.Volumes); err != nil {
		return err
	}

	if err := validateExternalVolumes(dev.ExternalVolumes); err != nil {
		return err
	}

	if _, err := resource.ParseQuantity(dev.PersistentVolumeSize()); err != nil {
		return fmt.Errorf("'persistentVolume.size' is not valid. A sample value would be '10Gi'")
	}

	for _, s := range dev.Services {
		if err := validatePullPolicy(s.ImagePullPolicy); err != nil {
			return err
		}
	}

	if dev.SSHServerPort <= 0 {
		return fmt.Errorf("'sshServerPort' must be > 0")
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

func validateVolumes(vList []Volume) error {
	for _, v := range vList {
		if !strings.HasPrefix(v.MountPath, "/") {
			return fmt.Errorf("volume relative paths are not supported")
		}
		if v.MountPath == "/" {
			return fmt.Errorf("mount path '/' is not supported")
		}
	}
	return nil
}

func validateExternalVolumes(vList []ExternalVolume) error {
	for _, v := range vList {
		if !strings.HasPrefix(v.MountPath, "/") {
			return fmt.Errorf("external volume '%s' mount path must be absolute", v.Name)
		}
		if v.MountPath == "/" {
			return fmt.Errorf("external volume '%s' mount path '/' is not supported", v.Name)
		}
	}
	return nil
}

//LoadRemote configures remote execution
func (dev *Dev) LoadRemote(pubKeyPath string) {
	if dev.RemotePort == 0 {
		p, err := GetAvailablePort()
		if err != nil {
			log.Infof("failed to get random port for SSH connection: %s", err)
			p = 2222
		}

		dev.RemotePort = p
		log.Infof("remote port not set, using %d", dev.RemotePort)
	}

	p := Secret{
		LocalPath:  pubKeyPath,
		RemotePath: authorizedKeysPath,
		Mode:       0600,
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
		log.Info(err)
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

func fullDevSubPath(subPath string) string {
	if subPath == "" {
		return SourceCodeSubPath
	}
	return path.Join(SourceCodeSubPath, subPath)
}

func fullGlobalSubPath(mountPath string) string {
	mountPath = strings.TrimSuffix(mountPath, "/")
	mountPath = ValidKubeNameRegex.ReplaceAllString(mountPath, "-")
	return fmt.Sprintf("volume%s", mountPath)
}

// ToTranslationRule translates a dev struct into a translation rule
func (dev *Dev) ToTranslationRule(main *Dev) *TranslationRule {
	rule := &TranslationRule{
		Container:        dev.Container,
		Image:            dev.Image,
		ImagePullPolicy:  dev.ImagePullPolicy,
		Environment:      dev.Environment,
		Secrets:          dev.Secrets,
		WorkDir:          dev.WorkDir,
		PersistentVolume: main.PersistentVolumeEnabled(),
		Volumes:          []VolumeMount{},
		SecurityContext:  dev.SecurityContext,
		Resources:        dev.Resources,
		Healthchecks:     dev.Healthchecks,
	}

	if main.PersistentVolumeEnabled() {
		rule.Volumes = append(
			rule.Volumes,
			VolumeMount{
				Name:      main.GetVolumeName(),
				MountPath: dev.MountPath,
				SubPath:   fullDevSubPath(dev.SubPath),
			},
		)
	}

	if main == dev {
		rule.Marker = dev.DevPath
		rule.Environment = append(
			rule.Environment,
			EnvVar{
				Name:  OktetoMarkerPathVariable,
				Value: path.Join(dev.MountPath, dev.DevPath),
			},
		)
		rule.Environment = append(
			rule.Environment,
			EnvVar{
				Name:  "OKTETO_NAMESPACE",
				Value: dev.Namespace,
			},
		)
		rule.Environment = append(
			rule.Environment,
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
		rule.Command = []string{"/var/okteto/bin/start.sh"}
		if main.RemoteModeEnabled() {
			rule.Args = []string{"-r"}
		} else {
			rule.Args = []string{}
		}
		for _, s := range rule.Secrets {
			rule.Args = append(rule.Args, "-s", fmt.Sprintf("%s:%s", s.GetFileName(), s.RemotePath))
		}
	} else if len(dev.Command.Values) > 0 {
		rule.Command = dev.Command.Values
		rule.Args = []string{}
	}

	for _, v := range dev.Volumes {
		var devSubPath string
		if v.SubPath == "" {
			devSubPath = fullGlobalSubPath(v.MountPath)
		} else {
			devSubPath = fullDevSubPath(v.SubPath)
		}
		rule.Volumes = append(
			rule.Volumes,
			VolumeMount{
				Name:      main.GetVolumeName(),
				MountPath: v.MountPath,
				SubPath:   devSubPath,
			},
		)
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

//UpdateNamespace updates the dev namespace
func (dev *Dev) UpdateNamespace(namespace string) error {
	if namespace == "" {
		return nil
	}
	if dev.Namespace != "" && dev.Namespace != namespace {
		return fmt.Errorf("the namespace in the okteto manifest '%s' does not match the namespace '%s'", dev.Namespace, namespace)
	}
	dev.Namespace = namespace
	return nil
}

//GevSandbox returns a deployment sandbox
func (dev *Dev) GevSandbox() *appsv1.Deployment {
	image := dev.Image
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
		return false
	}

	if dev.ExecuteOverSSHEnabled() {
		return true
	}

	if dev.RemotePort > 0 {
		return true
	}

	return len(dev.Reverse) > 0
}

// ExecuteOverSSHEnabled returns true if execute over SSH is enabled
func (dev *Dev) ExecuteOverSSHEnabled() bool {
	_, ok := os.LookupEnv("OKTETO_EXECUTE_SSH")
	return ok
}

// GetKeyName returns the secret key name
func (s *Secret) GetKeyName() string {
	return fmt.Sprintf("dev-secret-%s", filepath.Base(s.RemotePath))
}

// GetFileName returns the secret file name
func (s *Secret) GetFileName() string {
	return filepath.Base(s.RemotePath)
}

// PersistentVolumeEnabled returns true if persistent volumes are enabled for dev
func (dev *Dev) PersistentVolumeEnabled() bool {
	if dev.PersistentVolumeInfo == nil {
		return false
	}
	return dev.PersistentVolumeInfo.Enabled
}

// PersistentVolumeSize returns the persistent volume size
func (dev *Dev) PersistentVolumeSize() string {
	if dev.PersistentVolumeInfo == nil {
		return OktetoDefaultPVSize
	}
	if dev.PersistentVolumeInfo.Size == "" {
		return OktetoDefaultPVSize
	}
	return dev.PersistentVolumeInfo.Size
}

// PersistentVolumeStorageClass returns the persistent volume storage class
func (dev *Dev) PersistentVolumeStorageClass() string {
	if dev.PersistentVolumeInfo == nil {
		return ""
	}
	return dev.PersistentVolumeInfo.StorageClass
}
