package model

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	yaml "gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	oktetoPodNameTemplate      = "%s-0"
	oktetoStatefulSetTemplate  = "okteto-%s"
	oktetoVolumeNameTemplate   = "pvc-%d"
	oktetoAutoCreateAnnotation = "dev.okteto.com/auto-ingress"

	//OktetoInitContainer name of the okteto init container
	OktetoInitContainer = "okteto-init"

	//DefaultImage default image for sandboxes
	DefaultImage = "okteto/desk:0.1.5"
)

var (
	errBadName = fmt.Errorf("Invalid name: must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character")

	// ValidKubeNameRegex is the regex to validate a kubernetes resource name
	ValidKubeNameRegex = regexp.MustCompile(`[^a-z0-9\-]+`)

	devReplicas                      int32 = 1
	devTerminationGracePeriodSeconds int64
)

//Dev represents a cloud native development environment
type Dev struct {
	Name        string               `json:"name" yaml:"name"`
	Labels      map[string]string    `json:"labels,omitempty" yaml:"labels,omitempty"`
	Namespace   string               `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Container   string               `json:"container,omitempty" yaml:"container,omitempty"`
	Image       string               `json:"image,omitempty" yaml:"image,omitempty"`
	Environment []EnvVar             `json:"environment,omitempty" yaml:"environment,omitempty"`
	Command     []string             `json:"command,omitempty" yaml:"command,omitempty"`
	WorkDir     string               `json:"workdir" yaml:"workdir"`
	MountPath   string               `json:"mountpath,omitempty" yaml:"mountpath,omitempty"`
	SubPath     string               `json:"subpath,omitempty" yaml:"subpath,omitempty"`
	Volumes     []string             `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Forward     []Forward            `json:"forward,omitempty" yaml:"forward,omitempty"`
	Resources   ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	DevPath     string               `json:"-" yaml:"-"`
	DevDir      string               `json:"-" yaml:"-"`
	Services    []*Dev               `json:"services,omitempty" yaml:"services,omitempty"`
}

// EnvVar represents an environment value. When loaded, it will expand from the current env
type EnvVar struct {
	Name  string
	Value string
}

// Forward represents a port forwarding definition
type Forward struct {
	Local  int
	Remote int
}

// ResourceRequirements describes the compute resource requirements.
type ResourceRequirements struct {
	Limits   ResourceList
	Requests ResourceList
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
		Environment: make([]EnvVar, 0),
		Command:     make([]string, 0),
		Forward:     make([]Forward, 0),
		Volumes:     make([]string, 0),
		Resources: ResourceRequirements{
			Limits:   ResourceList{},
			Requests: ResourceList{},
		},
		Services: make([]*Dev, 0),
	}
	if err := yaml.Unmarshal(bytes, dev); err != nil {
		return nil, err
	}
	if err := dev.setDefaults(); err != nil {
		return nil, err
	}
	return dev, nil
}

func (dev *Dev) setDefaults() error {
	if len(dev.Command) == 0 {
		dev.Command = []string{"sh"}
	}
	if dev.MountPath == "" && dev.WorkDir == "" {
		dev.MountPath = "/okteto"
		dev.WorkDir = "/okteto"
	}
	if dev.WorkDir != "" && dev.MountPath == "" {
		dev.MountPath = dev.WorkDir
	}
	if dev.Labels == nil {
		dev.Labels = map[string]string{}
	}
	for _, s := range dev.Services {
		if s.MountPath == "" && s.WorkDir == "" {
			s.MountPath = "/okteto"
			s.WorkDir = "/okteto"
		}
		if s.WorkDir != "" && s.MountPath == "" {
			s.MountPath = s.WorkDir
		}
		if s.Labels == nil {
			s.Labels = map[string]string{}
		}
		if s.Name != "" && len(s.Labels) > 0 {
			return fmt.Errorf("'name' and 'labels' cannot be defined at the same time for service '%s'", s.Name)
		}
		s.Namespace = ""
		s.Forward = make([]Forward, 0)
		s.Volumes = make([]string, 0)
		s.Services = make([]*Dev, 0)
	}
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

	return nil
}

//GetStatefulSetName returns the syncthing statefulset name for a given dev environment
func (dev *Dev) GetStatefulSetName() string {
	n := fmt.Sprintf(oktetoStatefulSetTemplate, dev.Name)
	if len(n) > 52 {
		n = n[0:52]
	}
	return n
}

//GetPodName returns the syncthing statefulset pod name for a given dev environment
func (dev *Dev) GetPodName() string {
	n := dev.GetStatefulSetName()
	return fmt.Sprintf(oktetoPodNameTemplate, n)
}

//GetVolumeTemplateName returns the data volume name for a given dev environment
func (dev *Dev) GetVolumeTemplateName(i int) string {
	return fmt.Sprintf(oktetoVolumeNameTemplate, i)
}

//GetVolumeName returns the data volume name for a given dev environment
func (dev *Dev) GetVolumeName(i int) string {
	volumeName := dev.GetVolumeTemplateName(i)
	podName := dev.GetPodName()
	return fmt.Sprintf("%s-%s", volumeName, podName)
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
func (dev *Dev) ToTranslationRule(main *Dev, d *appsv1.Deployment, nodeName string) *TranslationRule {
	rule := &TranslationRule{
		Node:        nodeName,
		Container:   dev.Container,
		Image:       dev.Image,
		Environment: dev.Environment,
		WorkDir:     dev.WorkDir,
		Volumes: []VolumeMount{
			VolumeMount{
				Name:      main.GetVolumeName(0),
				MountPath: dev.MountPath,
				SubPath:   dev.SubPath,
			},
		},
		Resources: dev.Resources,
	}

	if main == dev {
		rule.Command = []string{"tail"}
		rule.Args = []string{"-f", "/dev/null"}
	} else if len(dev.Command) > 0 {
		rule.Command = dev.Command
		rule.Args = []string{}
	}

	for i, v := range dev.Volumes {
		rule.Volumes = append(
			rule.Volumes,
			VolumeMount{
				Name:      main.GetVolumeName(i + 1),
				MountPath: v,
			},
		)
	}
	return rule
}

//GevSandbox returns a deployment sandbox
func (dev *Dev) GevSandbox() *appsv1.Deployment {
	if dev.Image == "" {
		dev.Image = DefaultImage
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dev.Name,
			Namespace: dev.Namespace,
			Annotations: map[string]string{
				oktetoAutoCreateAnnotation: "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &devReplicas,
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
						apiv1.Container{
							Name:            "dev",
							Image:           dev.Image,
							ImagePullPolicy: apiv1.PullAlways,
							Command:         []string{"tail"},
							Args:            []string{"-f", "/dev/null"}},
					},
				},
			},
		},
	}
}
