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

package constants

import apiv1 "k8s.io/api/core/v1"

var (
	//OktetoBinImageTag image tag with okteto internal binaries
	OktetoBinImageTag = "okteto/bin:1.3.5"
)

const (

	// TimeFormat is the format to use when storing timestamps as a string
	TimeFormat = "2006-01-02T15:04:05"

	//Deployment k8s deployemnt kind
	Deployment = "Deployment"
	//StatefulSet k8s statefulset kind
	StatefulSet = "StatefulSet"

	//Localhost localhost
	Localhost = "localhost"
	//OktetoDefaultPVSize default volume size
	OktetoDefaultPVSize = "2Gi"
	//OktetoUpCmd up command
	OktetoUpCmd = "up"
	//OktetoPushCmd push command
	OktetoPushCmd                = "push"
	DefaultDinDImage             = "docker:20-dind"
	DefaultDockerHost            = "tcp://127.0.0.1:2376"
	DefaultDockerCertDir         = "/certs"
	DefaultDockerCacheDir        = "/var/lib/docker"
	DefaultDockerCertDirSubPath  = "certs"
	DefaultDockerCacheDirSubPath = "docker"

	//DeprecatedOktetoVolumeName name of the (deprecated) okteto persistent volume
	DeprecatedOktetoVolumeName = "okteto"
	//OktetoVolumeNameTemplate name template of the development container persistent volume
	OktetoVolumeNameTemplate = "%s-okteto"
	//DeprecatedOktetoVolumeNameTemplate name template of the development container persistent volume
	DeprecatedOktetoVolumeNameTemplate = "okteto-%s"
	//DataSubPath subpath in the development container persistent volume for the data volumes
	DataSubPath = "data"
	//SourceCodeSubPath subpath in the development container persistent volume for the source code
	SourceCodeSubPath = "src"
	//OktetoSyncthingMountPath syncthing volume mount path
	OktetoSyncthingMountPath = "/var/syncthing"
	//RemoteMountPath remote volume mount path
	RemoteMountPath = "/var/okteto/remote"
	//SyncthingSubPath subpath in the development container persistent volume for the syncthing data
	SyncthingSubPath = "syncthing"
	//DefaultSyncthingRescanInterval default syncthing re-scan interval
	DefaultSyncthingRescanInterval = 300
	//RemoteSubPath subpath in the development container persistent volume for the remote data
	RemoteSubPath = "okteto-remote"

	//OktetoInitContainer name of the okteto init container
	OktetoInitContainer = "okteto-init"

	//DefaultImage default image for sandboxes
	DefaultImage = "okteto/dev:latest"

	//ResourceAMDGPU amd.com/gpu resource
	ResourceAMDGPU apiv1.ResourceName = "amd.com/gpu"
	//ResourceNVIDIAGPU nvidia.com/gpu resource
	ResourceNVIDIAGPU apiv1.ResourceName = "nvidia.com/gpu"

	// AuthorizedKeysPath path is expected by remote
	AuthorizedKeysPath = "/var/okteto/remote/authorized_keys"

	//OktetoExtension identifies the okteto extension in kubeconfig files
	OktetoExtension = "okteto"

	// HelmSecretType indicates the type for secrets created by Helm
	HelmSecretType = "helm.sh/release.v1"
)
