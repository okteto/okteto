package model

import (
	"os"
	"sort"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/forward"
	yaml "gopkg.in/yaml.v2"
)

// DevRC represents the default properties for dev containers
type DevRC struct {
	Annotations          Annotations           `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Context              string                `json:"context,omitempty" yaml:"context,omitempty"`
	Command              Command               `json:"command,omitempty" yaml:"command,omitempty"`
	Environment          Environment           `json:"environment,omitempty" yaml:"environment,omitempty"`
	Forward              []forward.Forward     `json:"forward,omitempty" yaml:"forward,omitempty"`
	InitContainer        InitContainer         `json:"initContainer,omitempty" yaml:"initContainer,omitempty"`
	Labels               Labels                `json:"labels,omitempty" yaml:"labels,omitempty"`
	Metadata             *Metadata             `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Namespace            string                `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	PersistentVolumeInfo *PersistentVolumeInfo `json:"persistentVolume,omitempty" yaml:"persistentVolume,omitempty"`
	Resources            ResourceRequirements  `json:"resources,omitempty" yaml:"resources,omitempty"`
	Reverse              []Reverse             `json:"reverse,omitempty" yaml:"reverse,omitempty"`
	Selector             Selector              `json:"selector,omitempty" yaml:"selector,omitempty"`
	Secrets              []Secret              `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Sync                 Sync                  `json:"sync,omitempty" yaml:"sync,omitempty"`
	Timeout              Timeout               `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// GetRc returns a Dev object from a given file
func GetRc(devPath string) (*DevRC, error) {
	b, err := os.ReadFile(devPath)
	if err != nil {
		return nil, err
	}

	dev, err := ReadRC(b)
	if err != nil {
		return nil, err
	}

	return dev, nil
}

// ReadRC reads an okteto manifests
func ReadRC(bytes []byte) (*DevRC, error) {
	dev := &DevRC{}

	if bytes != nil {
		if err := yaml.UnmarshalStrict(bytes, dev); err != nil {
			return nil, err
		}
	}

	return dev, nil
}

func MergeDevWithDevRc(dev *Dev, devRc *DevRC) {
	if len(devRc.Annotations) > 0 {
		oktetoLog.Warning("The field 'annotations' is deprecated and will be removed in a future version. Use the field 'metadata.Annotations' instead (https://okteto.com/docs/reference/manifest/#metadata)")
		for annotationKey, annotationValue := range devRc.Annotations {
			dev.Metadata.Annotations[annotationKey] = annotationValue
		}
	}

	if devRc.Context != "" {
		dev.Context = devRc.Context
	}
	if len(devRc.Command.Values) != 0 {
		oktetoLog.Warning("Start command has been replaced with okteto developer file command")
		dev.Command.Values = devRc.Command.Values
	}

	for _, env := range devRc.Environment {
		idx := getEnvVarIdx(dev.Environment, env)
		if idx != -1 {
			dev.Environment[idx] = env
		} else {
			dev.Environment = append(dev.Environment, env)
		}

	}
	sort.SliceStable(dev.Environment, func(i, j int) bool {
		return strings.Compare(dev.Environment[i].Name, dev.Environment[j].Name) < 0
	})

	for _, fwd := range devRc.Forward {
		idx := getForwardPortIdx(dev.Forward, fwd)
		if idx != -1 {
			dev.Forward[idx] = fwd
		} else {
			dev.Forward = append(dev.Forward, fwd)
		}

	}
	sort.SliceStable(dev.Forward, func(i, j int) bool {
		return dev.Forward[i].Less(&dev.Forward[j])
	})

	if devRc.InitContainer.Image != "" {
		dev.InitContainer.Image = devRc.InitContainer.Image
	}
	for resourceKey, resourceValue := range devRc.InitContainer.Resources.Limits {
		dev.InitContainer.Resources.Limits[resourceKey] = resourceValue
	}
	for resourceKey, resourceValue := range devRc.InitContainer.Resources.Requests {
		dev.InitContainer.Resources.Requests[resourceKey] = resourceValue
	}

	if len(devRc.Labels) > 0 {
		oktetoLog.Warning("The field 'labels' is deprecated and will be removed in a future version. Use the field 'selector' instead (https://okteto.com/docs/reference/manifest/#selector)")
		for labelKey, labelValue := range devRc.Labels {
			dev.Selector[labelKey] = labelValue
		}
	}

	if devRc.Metadata != nil {
		for annotationKey, annotationValue := range devRc.Metadata.Annotations {
			dev.Metadata.Annotations[annotationKey] = annotationValue
		}
		for labelKey, labelValue := range devRc.Metadata.Labels {
			dev.Metadata.Labels[labelKey] = labelValue
		}
	}
	if devRc.Namespace != "" {
		dev.Namespace = devRc.Namespace
	}

	if devRc.PersistentVolumeInfo != nil && dev.PersistentVolumeInfo == nil {
		dev.PersistentVolumeInfo = devRc.PersistentVolumeInfo
	} else if devRc.PersistentVolumeInfo != nil && dev.PersistentVolumeInfo != nil {
		if devRc.PersistentVolumeInfo.Size != "" {
			dev.PersistentVolumeInfo.Size = devRc.PersistentVolumeInfo.Size
		}
		if devRc.PersistentVolumeInfo.StorageClass != "" {
			dev.PersistentVolumeInfo.StorageClass = devRc.PersistentVolumeInfo.StorageClass
		}
	}

	for resourceKey, resourceValue := range devRc.Resources.Limits {
		dev.Resources.Limits[resourceKey] = resourceValue
	}
	for resourceKey, resourceValue := range devRc.Resources.Requests {
		dev.Resources.Requests[resourceKey] = resourceValue
	}

	for _, rvs := range devRc.Reverse {
		idx := getReversePortIdx(dev.Reverse, rvs)
		if idx != -1 {
			dev.Reverse[idx] = rvs
		} else {
			dev.Reverse = append(dev.Reverse, rvs)
		}

	}
	sort.SliceStable(dev.Reverse, func(i, j int) bool {
		return dev.Reverse[i].Local < dev.Reverse[j].Local
	})

	dev.Secrets = append(dev.Secrets, devRc.Secrets...)

	for key, value := range devRc.Selector {
		dev.Selector[key] = value
	}

	if !devRc.Sync.Compression {
		dev.Sync.Compression = false
	}
	if devRc.Sync.Verbose {
		dev.Sync.Verbose = devRc.Sync.Verbose
	}
	if devRc.Sync.RescanInterval != 0 {
		dev.Sync.RescanInterval = devRc.Sync.RescanInterval
	}

	dev.Sync.Folders = append(dev.Sync.Folders, devRc.Sync.Folders...)

	if devRc.Timeout.Default != 0 {
		dev.Timeout.Default = devRc.Timeout.Default
	}
	if devRc.Timeout.Resources != 0 {
		dev.Timeout.Resources = devRc.Timeout.Resources
	}
}

func getEnvVarIdx(environment Environment, envVar EnvVar) int {
	idx := -1
	for aux, env := range environment {
		if env.Name == envVar.Name {
			return aux
		}
	}
	return idx
}

func getForwardPortIdx(forwardList []forward.Forward, forward forward.Forward) int {
	idx := -1
	for aux, fwd := range forwardList {
		if fwd.Remote == forward.Remote {
			return aux
		}
	}
	return idx
}

func getReversePortIdx(reverseList []Reverse, reverse Reverse) int {
	idx := -1
	for aux, rvrs := range reverseList {
		if rvrs.Remote == reverse.Remote {
			return aux
		}
	}
	return idx
}
