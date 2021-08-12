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

package model

import apiv1 "k8s.io/api/core/v1"

const (
	//Version represents the current dev data version
	Version = "1.0"

	// TimeFormat is the format to use when storing timestamps as a string
	TimeFormat = "2006-01-02T15:04:05"

	// DevLabel indicates the dev pod
	DevLabel = "dev.okteto.com"

	// InteractiveDevLabel indicates the interactive dev pod
	InteractiveDevLabel = "interactive.dev.okteto.com"

	// DetachedDevLabel indicates the detached dev pods
	DetachedDevLabel = "detached.dev.okteto.com"

	// RevisionAnnotation indicates the revision when the development container was activated
	RevisionAnnotation = "dev.okteto.com/revision"

	// DeploymentAnnotation indicates the original deployment manifest  when the development container was activated
	DeploymentAnnotation = "dev.okteto.com/deployment"

	// StatefulsetAnnotation indicates the original statefulset manifest  when the development container was activated
	StatefulsetAnnotation = "dev.okteto.com/statefulset"

	// LastBuiltAnnotation indicates the timestamp of an operation
	LastBuiltAnnotation = "dev.okteto.com/last-built"

	// TranslationAnnotation sets the translation rules
	TranslationAnnotation = "dev.okteto.com/translation"

	// SyncLabel indicates a syncthing pod
	SyncLabel = "syncthing.okteto.com"

	//OktetoRepositoryAnnotation indicates the git repo url with the source code of this component
	OktetoRepositoryAnnotation = "dev.okteto.com/repository"

	//OktetoPathAnnotation indicates the okteto manifest path of this component
	OktetoPathAnnotation = "dev.okteto.com/path"

	//FluxAnnotation indicates if the deployment ha been deployed by Flux
	FluxAnnotation = "helm.fluxcd.io/antecedent"

	//DefaultStorageClassAnnotation indicates the defaault storage class
	DefaultStorageClassAnnotation = "storageclass.kubernetes.io/is-default-class"

	//StateBeforeSleepingAnnontation indicates the state of the resource prior to scale it to zero
	StateBeforeSleepingAnnontation = "dev.okteto.com/state-before-sleeping"

	// DeployedByLabel indicates the service account that deployed an object
	DeployedByLabel = "dev.okteto.com/deployed-by"

	// StackLabel indicates the object is a stack
	StackLabel = "stack.okteto.com"

	// StackNameLabel indicates the name of the stack an object belongs to
	StackNameLabel = "stack.okteto.com/name"

	// StackServiceNameLabel indicates the name of the stack service an object belongs to
	StackServiceNameLabel = "stack.okteto.com/service"

	// StackEndpointNameLabel indicates the name of the endpoint an object belongs to
	StackEndpointNameLabel = "stack.okteto.com/endpoint"

	// StackIngressAutoGenerateHost generates a ingress host for
	OktetoIngressAutoGenerateHost = "dev.okteto.com/generate-host"

	// OktetoAutoIngressAnnotation indicates an ingress must be created for a service
	OktetoAutoIngressAnnotation = "dev.okteto.com/auto-ingress"

	// OktetoInstallerRunningLabel indicates the okteto installer is running on this resource
	OktetoInstallerRunningLabel = "dev.okteto.com/installer-running"

	// StackVolumeNameLabel indicates the name of the stack volume an object belongs to
	StackVolumeNameLabel = "stack.okteto.com/volume"

	//Localhost localhost
	Localhost                   = "localhost"
	oktetoSSHServerPortVariable = "OKTETO_REMOTE_PORT"
	oktetoDefaultSSHServerPort  = 2222
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
	OktetoVolumeNameTemplate = "okteto-%s"
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
	//OktetoURLAnnotation indicates the okteto cluster public url
	OktetoURLAnnotation = "dev.okteto.com/url"
	//OktetoAutoCreateAnnotation indicates if the deployment was auto generatted by okteto up
	OktetoAutoCreateAnnotation = "dev.okteto.com/auto-create"
	//OktetoRestartAnnotation indicates the dev pod must be recreated to pull the latest version of its image
	OktetoRestartAnnotation = "dev.okteto.com/restart"
	//OktetoStignoreAnnotation indicates the hash of the stignore files to force redeployment
	OktetoStignoreAnnotation = "dev.okteto.com/stignore"
	//OktetoDivertLabel indicates the object is a diverted version
	OktetoDivertLabel = "dev.okteto.com/divert"
	//OktetoDivertServiceModificationAnnotation indicates the service modification done by diverting a service
	OktetoDivertServiceModificationAnnotation = "divert.okteto.com/modification"
	//OktetoInjectTokenAnnotation annotation to inject the okteto token
	OktetoInjectTokenAnnotation = "dev.okteto.com/inject-token"

	//OktetoInitContainer name of the okteto init container
	OktetoInitContainer = "okteto-init"

	//DefaultImage default image for sandboxes
	DefaultImage = "okteto/dev:latest"

	//TranslationVersion version of the translation schema
	TranslationVersion = "1.0"

	//ResourceAMDGPU amd.com/gpu resource
	ResourceAMDGPU apiv1.ResourceName = "amd.com/gpu"
	//ResourceNVIDIAGPU nvidia.com/gpu resource
	ResourceNVIDIAGPU apiv1.ResourceName = "nvidia.com/gpu"

	// this path is expected by remote
	authorizedKeysPath = "/var/okteto/remote/authorized_keys"

	syncFieldDocsURL = "https://okteto.com/docs/reference/manifest/#sync-string-required"
)
