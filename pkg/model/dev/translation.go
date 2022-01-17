// Copyright 2021 The Okteto Authors
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

package dev

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/okteto/okteto/pkg/model/constants"
	"github.com/okteto/okteto/pkg/model/environment"
	"github.com/okteto/okteto/pkg/model/metadata"
	"github.com/okteto/okteto/pkg/model/secrets"
	apiv1 "k8s.io/api/core/v1"
)

// TranslationRule represents how to apply a container translation in a deployment
type TranslationRule struct {
	Marker            string                  `json:"marker"`
	OktetoBinImageTag string                  `json:"oktetoBinImageTag"`
	Node              string                  `json:"node,omitempty"`
	Container         string                  `json:"container,omitempty"`
	Image             string                  `json:"image,omitempty"`
	Labels            metadata.Labels         `json:"labels,omitempty"`
	ImagePullPolicy   apiv1.PullPolicy        `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	Environment       environment.Environment `json:"environment,omitempty"`
	Secrets           []secrets.Secret        `json:"secrets,omitempty"`
	Command           []string                `json:"command,omitempty"`
	Args              []string                `json:"args,omitempty"`
	WorkDir           string                  `json:"workdir"`
	Healthchecks      bool                    `json:"healthchecks" yaml:"healthchecks"`
	PersistentVolume  bool                    `json:"persistentVolume" yaml:"persistentVolume"`
	Volumes           []VolumeMount           `json:"volumes,omitempty"`
	SecurityContext   *SecurityContext        `json:"securityContext,omitempty"`
	ServiceAccount    string                  `json:"serviceAccount,omitempty" yaml:"serviceAccount,omitempty"`
	Resources         ResourceRequirements    `json:"resources,omitempty"`
	InitContainer     InitContainer           `json:"initContainers,omitempty"`
	Probes            *Probes                 `json:"probes" yaml:"probes"`
	Lifecycle         *Lifecycle              `json:"lifecycle" yaml:"lifecycle"`
	Docker            DinDContainer           `json:"docker" yaml:"docker"`
	NodeSelector      map[string]string       `json:"nodeSelector" yaml:"nodeSelector"`
	Affinity          *apiv1.Affinity         `json:"affinity" yaml:"affinity"`
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
	return v.SubPath == constants.SyncthingSubPath && v.MountPath == constants.OktetoSyncthingMountPath
}

// ToTranslationRule translates a dev struct into a translation rule
func (dev *Dev) ToTranslationRule(main *Dev, reset bool) *TranslationRule {
	rule := &TranslationRule{
		Container:        dev.Container,
		ImagePullPolicy:  dev.ImagePullPolicy,
		Environment:      dev.Environment,
		Secrets:          dev.Secrets,
		WorkDir:          dev.Workdir,
		PersistentVolume: main.PersistentVolumeEnabled(),
		Docker:           main.Docker,
		Volumes:          []VolumeMount{},
		SecurityContext:  dev.SecurityContext,
		ServiceAccount:   dev.ServiceAccount,
		Resources:        dev.Resources,
		Healthchecks:     dev.Healthchecks,
		InitContainer:    dev.InitContainer,
		Probes:           dev.Probes,
		Lifecycle:        dev.Lifecycle,
		NodeSelector:     dev.NodeSelector,
		Affinity:         (*apiv1.Affinity)(dev.Affinity),
	}

	if !dev.EmptyImage {
		rule.Image = dev.Image.Name
	}

	if rule.Healthchecks {
		rule.Probes = &Probes{Liveness: true, Startup: true, Readiness: true}
	}

	if areProbesEnabled(rule.Probes) {
		rule.Healthchecks = true
	}
	if main == dev {
		rule.Marker = constants.OktetoBinImageTag //for backward compatibility
		rule.OktetoBinImageTag = dev.InitContainer.Image
		rule.Environment = append(
			rule.Environment,
			environment.EnvVar{
				Name:  "OKTETO_NAMESPACE",
				Value: dev.Namespace,
			},
			environment.EnvVar{
				Name:  "OKTETO_NAME",
				Value: dev.Name,
			},
		)
		if dev.Username != "" {
			rule.Environment = append(
				rule.Environment,
				environment.EnvVar{
					Name:  "OKTETO_USERNAME",
					Value: dev.Username,
				},
			)
		}
		if dev.Docker.Enabled {
			rule.Environment = append(
				rule.Environment,
				environment.EnvVar{
					Name:  "OKTETO_REGISTRY_URL",
					Value: dev.RegistryURL,
				},
				environment.EnvVar{
					Name:  "DOCKER_HOST",
					Value: constants.DefaultDockerHost,
				},
				environment.EnvVar{
					Name:  "DOCKER_CERT_PATH",
					Value: "/certs/client",
				},
				environment.EnvVar{
					Name:  "DOCKER_TLS_VERIFY",
					Value: "1",
				},
			)
			rule.Volumes = append(
				rule.Volumes,
				VolumeMount{
					Name:      main.GetVolumeName(),
					MountPath: constants.DefaultDockerCertDir,
					SubPath:   constants.DefaultDockerCertDirSubPath,
				},
				VolumeMount{
					Name:      main.GetVolumeName(),
					MountPath: constants.DefaultDockerCacheDir,
					SubPath:   constants.DefaultDockerCacheDirSubPath,
				},
			)
		}

		// We want to minimize environment mutations, so only reconfigure the SSH
		// server port if a non-default is specified.
		if dev.SSHServerPort != constants.OktetoDefaultSSHServerPort {
			rule.Environment = append(
				rule.Environment,
				environment.EnvVar{
					Name:  constants.OktetoSSHServerPortVariableEnvVar,
					Value: strconv.Itoa(dev.SSHServerPort),
				},
			)
		}
		rule.Volumes = append(
			rule.Volumes,
			VolumeMount{
				Name:      main.GetVolumeName(),
				MountPath: constants.OktetoSyncthingMountPath,
				SubPath:   constants.SyncthingSubPath,
			},
		)
		if main.RemoteModeEnabled() {
			rule.Volumes = append(
				rule.Volumes,
				VolumeMount{
					Name:      main.GetVolumeName(),
					MountPath: constants.RemoteMountPath,
					SubPath:   constants.RemoteSubPath,
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
		if dev.Docker.Enabled {
			rule.Args = append(rule.Args, "-d")
		}
	} else if len(dev.Command.Values) > 0 {
		rule.Command = dev.Command.Values
		rule.Args = []string{}
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

func areProbesEnabled(probes *Probes) bool {
	if probes != nil {
		return probes.Liveness || probes.Readiness || probes.Startup
	}
	return false
}

func areAllProbesEnabled(probes *Probes) bool {
	if probes != nil {
		return probes.Liveness && probes.Readiness && probes.Startup
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

	if v, ok := os.LookupEnv(constants.OktetoExecuteSSHEnvVar); ok && v == "false" {
		return false
	}
	return true
}
