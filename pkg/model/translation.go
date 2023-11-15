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
	"github.com/okteto/okteto/pkg/env"
	apiv1 "k8s.io/api/core/v1"
)

// TranslationRule represents how to apply a container translation in a deployment
type TranslationRule struct {
	InitContainer     InitContainer        `json:"initContainers,omitempty"`
	Resources         ResourceRequirements `json:"resources,omitempty"`
	SecurityContext   *SecurityContext     `json:"securityContext,omitempty"`
	Probes            *Probes              `json:"probes" yaml:"probes"`
	Lifecycle         *Lifecycle           `json:"lifecycle" yaml:"lifecycle"`
	Labels            Labels               `json:"labels,omitempty"`
	NodeSelector      map[string]string    `json:"nodeSelector" yaml:"nodeSelector"`
	Affinity          *apiv1.Affinity      `json:"affinity" yaml:"affinity"`
	ServiceAccount    string               `json:"serviceAccount,omitempty" yaml:"serviceAccount,omitempty"`
	WorkDir           string               `json:"workdir"`
	Marker            string               `json:"marker"`
	OktetoBinImageTag string               `json:"oktetoBinImageTag"`
	Node              string               `json:"node,omitempty"`
	Container         string               `json:"container,omitempty"`
	Image             string               `json:"image,omitempty"`
	ImagePullPolicy   apiv1.PullPolicy     `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	Environment       env.Environment      `json:"environment,omitempty"`
	Secrets           []Secret             `json:"secrets,omitempty"`
	Command           []string             `json:"command,omitempty"`
	Args              []string             `json:"args,omitempty"`
	Volumes           []VolumeMount        `json:"volumes,omitempty"`
	Healthchecks      bool                 `json:"healthchecks" yaml:"healthchecks"`
	PersistentVolume  bool                 `json:"persistentVolume" yaml:"persistentVolume"`
}

// IsMainDevContainer returns true if the translation rule applies to the main dev container of the okteto manifest
func (r *TranslationRule) IsMainDevContainer() bool {
	return r.OktetoBinImageTag != ""
}

// VolumeMount represents a volume mount
type VolumeMount struct {
	Name      string `json:"name,omitempty"`
	MountPath string `json:"mountpath,omitempty"`
	SubPath   string `json:"subpath,omitempty"`
}

// IsSyncthing returns the volume mount is for syncthing
func (v *VolumeMount) IsSyncthing() bool {
	return v.SubPath == SyncthingSubPath && v.MountPath == OktetoSyncthingMountPath
}
