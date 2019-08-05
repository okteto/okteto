package model

import (
	appsv1 "k8s.io/api/apps/v1"
)

//Translation represents the information for translating a deployment
type Translation struct {
	Interactive bool               `json:"interactive"`
	Name        string             `json:"name"`
	Deployment  *appsv1.Deployment `json:"-"`
	Replicas    int32              `json:"replicas"`
	Rules       []*TranslationRule `json:"rules"`
}

//TranslationRule represents how to apply a container translation in a deployment
type TranslationRule struct {
	Node         string               `json:"node,omitempty"`
	Container    string               `json:"container,omitempty"`
	Image        string               `json:"image,omitempty"`
	Environment  []EnvVar             `json:"environment,omitempty"`
	Command      []string             `json:"command,omitempty"`
	Args         []string             `json:"args,omitempty"`
	WorkDir      string               `json:"workdir"`
	Healthchecks bool                 `json:"healthchecks" yaml:"healthchecks"`
	Volumes      []VolumeMount        `json:"volumes,omitempty"`
	Resources    ResourceRequirements `json:"resources,omitempty"`
}

//VolumeMount represents a volume mount
type VolumeMount struct {
	Name      string `json:"name,omitempty"`
	MountPath string `json:"mountpath,omitempty"`
	SubPath   string `json:"subpath,omitempty"`
}
