package model

import (
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

//Translation represents the information for translating a deployment
type Translation struct {
	Interactive bool               `json:"interactive"`
	Name        string             `json:"name"`
	Marker      string             `json:"marker"`
	Version     string             `json:"version"`
	Deployment  *appsv1.Deployment `json:"-"`
	Replicas    int32              `json:"replicas"`
	Rules       []*TranslationRule `json:"rules"`
}

//TranslationRule represents how to apply a container translation in a deployment
type TranslationRule struct {
	Container       string               `json:"container,omitempty"`
	Image           string               `json:"image,omitempty"`
	ImagePullPolicy apiv1.PullPolicy     `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	Environment     []EnvVar             `json:"environment,omitempty"`
	Command         []string             `json:"command,omitempty"`
	Args            []string             `json:"args,omitempty"`
	WorkDir         string               `json:"workdir"`
	Healthchecks    bool                 `json:"healthchecks" yaml:"healthchecks"`
	Volumes         []VolumeMount        `json:"volumes,omitempty"`
	SecurityContext *SecurityContext     `json:"securityContext,omitempty"`
	Resources       ResourceRequirements `json:"resources,omitempty"`
}

//VolumeMount represents a volume mount
type VolumeMount struct {
	Name      string `json:"name,omitempty"`
	MountPath string `json:"mountpath,omitempty"`
	SubPath   string `json:"subpath,omitempty"`
}
