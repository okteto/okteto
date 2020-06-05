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
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

//Translation represents the information for translating a deployment
type Translation struct {
	Interactive bool               `json:"interactive"`
	Name        string             `json:"name"`
	Version     string             `json:"version"`
	Deployment  *appsv1.Deployment `json:"-"`
	Annotations map[string]string  `json:"annotations,omitempty"`
	Tolerations []apiv1.Toleration `json:"tolerations,omitempty"`
	Replicas    int32              `json:"replicas"`
	Rules       []*TranslationRule `json:"rules"`
}

//TranslationRule represents how to apply a container translation in a deployment
type TranslationRule struct {
	Marker           string               `json:"marker"`
	Node             string               `json:"node,omitempty"`
	Container        string               `json:"container,omitempty"`
	Image            string               `json:"image,omitempty"`
	ImagePullPolicy  apiv1.PullPolicy     `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	Environment      []EnvVar             `json:"environment,omitempty"`
	Secrets          []Secret             `json:"secrets,omitempty"`
	Command          []string             `json:"command,omitempty"`
	Args             []string             `json:"args,omitempty"`
	WorkDir          string               `json:"workdir"`
	Healthchecks     bool                 `json:"healthchecks" yaml:"healthchecks"`
	PersistentVolume bool                 `json:"persistentVolume" yaml:"persistentVolume"`
	Volumes          []VolumeMount        `json:"volumes,omitempty"`
	SecurityContext  *SecurityContext     `json:"securityContext,omitempty"`
	Resources        ResourceRequirements `json:"resources,omitempty"`
}

//VolumeMount represents a volume mount
type VolumeMount struct {
	Name      string `json:"name,omitempty"`
	MountPath string `json:"mountpath,omitempty"`
	SubPath   string `json:"subpath,omitempty"`
}

//IsSyncthing returns the volume mount is for syncthing
func (v *VolumeMount) IsSyncthing() bool {
	return v.SubPath == SyncthingSubPath && v.MountPath == OktetoSyncthingMountPath
}
