package model

import (
	appsv1 "k8s.io/api/apps/v1"
)

//Translation represents the information for translating a deployment
type Translation struct {
	Interactive bool               `json:"-" yaml:"-"`
	Name        string             `json:"-" yaml:"-"`
	Deployment  *appsv1.Deployment `json:"-" yaml:"-"`
	Rules       []*TranslationRule `json:"rules" yaml:"rules"`
}

//TranslationRule represents how to apply a container translation in a deployment
type TranslationRule struct {
	Node        string               `json:"node,omitempty" yaml:"node,omitempty"`
	Container   string               `json:"container,omitempty" yaml:"container,omitempty"`
	Image       string               `json:"image,omitempty" yaml:"image,omitempty"`
	Environment []EnvVar             `json:"environment,omitempty" yaml:"environment,omitempty"`
	Command     []string             `json:"command,omitempty" yaml:"command,omitempty"`
	Args        []string             `json:"args,omitempty" yaml:"args,omitempty"`
	WorkDir     string               `json:"workdir" yaml:"workdir"`
	Volumes     []VolumeMount        `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Resources   ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
}

//VolumeMount represents a volume mount
type VolumeMount struct {
	Name      string `json:"name,omitempty" yaml:"name,omitempty"`
	MountPath string `json:"mountpath,omitempty" yaml:"mountpath,omitempty"`
	SubPath   string `json:"subpath,omitempty" yaml:"subpath,omitempty"`
}
