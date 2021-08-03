package model

import (
	"errors"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/okteto/okteto/pkg/log"
	yaml "gopkg.in/yaml.v2"
)

// DevRC represents the default properties for dev containers
type DevRC struct {
	Annotations          Annotations           `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Context              string                `json:"context,omitempty" yaml:"context,omitempty"`
	Command              Command               `json:"command,omitempty" yaml:"command,omitempty"`
	Docker               DinDContainer         `json:"docker,omitempty" yaml:"docker,omitempty"`
	Environment          Environment           `json:"environment,omitempty" yaml:"environment,omitempty"`
	Forward              []Forward             `json:"forward,omitempty" yaml:"forward,omitempty"`
	InitContainer        InitContainer         `json:"initContainer,omitempty" yaml:"initContainer,omitempty"`
	Labels               Labels                `json:"labels,omitempty" yaml:"labels,omitempty"`
	Namespace            string                `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	PersistentVolumeInfo *PersistentVolumeInfo `json:"persistentVolume,omitempty" yaml:"persistentVolume,omitempty"`
	Resources            ResourceRequirements  `json:"resources,omitempty" yaml:"resources,omitempty"`
	Reverse              []Reverse             `json:"reverse,omitempty" yaml:"reverse,omitempty"`
	Secrets              []Secret              `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Sync                 Sync                  `json:"sync,omitempty" yaml:"sync,omitempty"`
	Timeout              Timeout               `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// Get returns a Dev object from a given file
func GetRc(devPath string) (*DevRC, error) {
	b, err := ioutil.ReadFile(devPath)
	if err != nil {
		return nil, err
	}

	dev, err := ReadRC(b)
	if err != nil {
		return nil, err
	}

	return dev, nil
}

// Read reads an okteto manifests
func ReadRC(bytes []byte) (*DevRC, error) {
	dev := &DevRC{}

	if bytes != nil {
		if err := yaml.UnmarshalStrict(bytes, dev); err != nil {
			if strings.HasPrefix(err.Error(), "yaml: unmarshal errors:") {
				var sb strings.Builder
				_, _ = sb.WriteString("Invalid developer level manifest:\n")
				l := strings.Split(err.Error(), "\n")
				for i := 1; i < len(l); i++ {
					e := strings.TrimSuffix(l[i], "in type model.DevRC")
					e = strings.TrimSpace(e)
					_, _ = sb.WriteString(fmt.Sprintf("    - %s\n", e))
				}

				_, _ = sb.WriteString("    See https://okteto.com/docs/reference/manifest for details")
				return nil, errors.New(sb.String())
			}

			msg := strings.Replace(err.Error(), "yaml: unmarshal errors:", "invalid developer level manifest:", 1)
			msg = strings.TrimSuffix(msg, "in type model.DevRC")
			return nil, errors.New(msg)
		}
	}

	return dev, nil
}

func MergeDevWithDevRc(dev *Dev, devRc *DevRC) {
	for annotationKey, annotationValue := range devRc.Annotations {
		dev.Annotations[annotationKey] = annotationValue
	}

	if devRc.Context != "" {
		dev.Context = devRc.Context
	}
	if len(devRc.Command.Values) != 0 {
		log.Warning("Start command has been replaced with okteto developer file command")
		dev.Command.Values = devRc.Command.Values
	}

	if devRc.Docker.Enabled {
		dev.Docker.Enabled = devRc.Docker.Enabled
	}
	if devRc.Docker.Image != "" {
		dev.Docker.Image = devRc.Docker.Image
	}
	for resourceKey, resourceValue := range devRc.Docker.Resources.Limits {
		dev.Docker.Resources.Limits[resourceKey] = resourceValue
	}
	for resourceKey, resourceValue := range devRc.Docker.Resources.Requests {
		dev.Docker.Resources.Requests[resourceKey] = resourceValue
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
		return dev.Forward[i].less(&dev.Forward[j])
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

	for labelKey, labelValue := range devRc.Labels {
		dev.Labels[labelKey] = labelValue
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

	for _, secret := range devRc.Secrets {
		dev.Secrets = append(dev.Secrets, secret)
	}

	if devRc.Sync.Compression {
		dev.Sync.Compression = devRc.Sync.Compression
	}
	if devRc.Sync.Verbose {
		dev.Sync.Verbose = devRc.Sync.Verbose
	}
	if devRc.Sync.RescanInterval != 0 {
		dev.Sync.RescanInterval = devRc.Sync.RescanInterval
	}
	for _, folder := range devRc.Sync.Folders {
		dev.Sync.Folders = append(dev.Sync.Folders, folder)
	}

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

func getForwardPortIdx(forwardList []Forward, forward Forward) int {
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
