// Copyright 2022 The Okteto Authors
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
	// TimeFormat is the format to use when storing timestamps as a string
	TimeFormat = "2006-01-02T15:04:05"

	// DevLabel indicates the deployment is in dev mode
	DevLabel = "dev.okteto.com"

	// DevCloneLabel indicates it is a dev pod clone
	DevCloneLabel = "dev.okteto.com/clone"

	// AppReplicasAnnotation indicates the number of replicas before dev mode was activated
	AppReplicasAnnotation = "dev.okteto.com/replicas"

	// InteractiveDevLabel indicates the interactive dev pod
	InteractiveDevLabel = "interactive.dev.okteto.com"

	// OktetoSampleAnnotation indicates that the repo is a okteto sample
	OktetoSampleAnnotation = "dev.okteto.com/sample"

	// DetachedDevLabel indicates the detached dev pods
	DetachedDevLabel = "detached.dev.okteto.com"

	// DeploymentRevisionAnnotation indicates the revision when the development container was activated
	DeploymentRevisionAnnotation = "deployment.kubernetes.io/revision"

	// OktetoRevisionAnnotation indicates the revision when the development container was activated
	OktetoRevisionAnnotation = "dev.okteto.com/revision"

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

	// GitDeployLabel indicates the object is an app
	GitDeployLabel = "dev.okteto.com/git-deploy"

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

	// OktetoPrivateSvcAnnotation indicates an ingress must be created private
	OktetoPrivateSvcAnnotation = "dev.okteto.com/private"

	// OktetoInstallerRunningLabel indicates the okteto installer is running on this resource
	OktetoInstallerRunningLabel = "dev.okteto.com/installer-running"

	// StackVolumeNameLabel indicates the name of the stack volume an object belongs to
	StackVolumeNameLabel = "stack.okteto.com/volume"

	//Deployment k8s deployemnt kind
	Deployment = "Deployment"
	//StatefulSet k8s statefulset kind
	StatefulSet = "StatefulSet"

	//Localhost localhost
	Localhost = "localhost"
	//PrivilegedLocalhost localhost
	PrivilegedLocalhost         = "0.0.0.0"
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
	//OktetoURLAnnotation indicates the okteto cluster public url
	OktetoURLAnnotation = "dev.okteto.com/url"
	//OktetoAutoCreateAnnotation indicates if the deployment was auto generatted by okteto up
	OktetoAutoCreateAnnotation = "dev.okteto.com/auto-create"
	//OktetoRestartAnnotation indicates the dev pod must be recreated to pull the latest version of its image
	OktetoRestartAnnotation = "dev.okteto.com/restart"
	//OktetoSyncAnnotation indicates the hash of the sync folders to force redeployment
	OktetoSyncAnnotation = "dev.okteto.com/sync"
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

	//ResourceAMDGPU amd.com/gpu resource
	ResourceAMDGPU apiv1.ResourceName = "amd.com/gpu"
	//ResourceNVIDIAGPU nvidia.com/gpu resource
	ResourceNVIDIAGPU apiv1.ResourceName = "nvidia.com/gpu"

	// this path is expected by remote
	authorizedKeysPath = "/var/okteto/remote/authorized_keys"

	syncFieldDocsURL = "https://okteto.com/docs/reference/manifest/#sync-string-required"

	//OktetoExtension identifies the okteto extension in kubeconfig files
	OktetoExtension = "okteto"

	// HelmSecretType indicates the type for secrets created by Helm
	HelmSecretType = "helm.sh/release.v1"

	// OktetoGitBranchEnvVar is the name of the Git branch currently being deployed.
	OktetoGitBranchEnvVar = "OKTETO_GIT_BRANCH"

	// OktetoGitCommitEnvVar is the SHA1 hash of the last commit of the branch.
	OktetoGitCommitEnvVar = "OKTETO_GIT_COMMIT"

	// OktetoRegistryURLEnvVar is the url of the Okteto Registry
	OktetoRegistryURLEnvVar = "OKTETO_REGISTRY_URL"

	// OktetoBuildkitHostURLEnvVar is the url of the Okteto Buildkit instance
	OktetoBuildkitHostURLEnvVar = "BUILDKIT_HOST"

	// OktetoBinEnvVar defines the okteto binary that should be used
	OktetoBinEnvVar = "OKTETO_BIN"

	// OktetoSkipCleanupEnvVar defines the okteto binary that should be used
	OktetoSkipCleanupEnvVar = "OKTETO_SKIP_CLEANUP"

	// OktetoUserEnvVar defines the user is using okteto
	OktetoUserEnvVar = "OKTETO_USER"

	// OktetoUserNameEnvVar defines the user is using okteto
	OktetoUserNameEnvVar = "OKTETO_USERNAME"

	// OktetoTokenEnvVar defines the environmental variable that stores the okteto user token
	OktetoTokenEnvVar = "OKTETO_TOKEN"

	// OktetoURLEnvVar defines the cluster url the user is using
	OktetoURLEnvVar = "OKTETO_URL"

	// OktetoContextEnvVar defines the k8s context the user is using
	OktetoContextEnvVar = "OKTETO_CONTEXT"

	// OktetoNamespaceEnvVar defines the namespace the user is using
	OktetoNamespaceEnvVar = "OKTETO_NAMESPACE"

	// OktetoLanguageEnvVar defines the language of the dev
	OktetoLanguageEnvVar = "OKTETO_LANGUAGE"

	// SyncthingVersionEnvVar defines the syncthing version okteto should use
	SyncthingVersionEnvVar = "OKTETO_SYNCTHING_VERSION"

	// OktetoSkipContextTest if set skips the context test
	OktetoSkipContextTestEnvVar = "OKTETO_SKIP_CONTEXT_TEST"

	// OktetoAutoDeployEnvVar if set the application will be deployed while running okteto up
	OktetoAutoDeployEnvVar = "OKTETO_AUTODEPLOY"

	// OktetoAppsSubdomainEnvVar defines which is the subdomain for urls
	OktetoAppsSubdomainEnvVar = "OKTETO_APPS_SUBDOMAIN"

	// OktetoPathEnvVar defines where is okteto binary
	OktetoPathEnvVar = "OKTETO_PATH"

	// OktetoOriginEnvVar defines where is executing okteto
	OktetoOriginEnvVar = "OKTETO_ORIGIN"

	// OktetoFolderEnvVar defines the path of okteto folder
	OktetoFolderEnvVar = "OKTETO_FOLDER"

	// OktetoFolderEnvVar defines the path of okteto folder
	OktetoHomeEnvVar = "OKTETO_HOME"

	// OktetoExecuteSSHEnvVar defines if the command should be executed through ssh
	OktetoExecuteSSHEnvVar = "OKTETO_EXECUTE_SSH"

	// OktetoNameEnvVar defines if the command is running inside okteot
	OktetoNameEnvVar = "OKTETO_NAME"

	// OktetoKubernetesTimeoutEnvVar defines the timeout for kubernetes operations
	OktetoKubernetesTimeoutEnvVar = "OKTETO_KUBERNETES_TIMEOUT"

	// OktetoDisableSpinnerEnvVar if true spinner is disabled
	OktetoDisableSpinnerEnvVar = "OKTETO_DISABLE_SPINNER"

	// OktetoRescanIntervalEnvVar defines the time between scans for syncthing
	OktetoRescanIntervalEnvVar = "OKTETO_RESCAN_INTERVAL"

	// OktetoWithinDeployCommandContextEnvVar defines if an okteto command is executed by deploy command
	OktetoWithinDeployCommandContextEnvVar = "OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT"

	// OktetoTimeoutEnvVar defines the timeout for okteto commands
	OktetoTimeoutEnvVar = "OKTETO_TIMEOUT"

	// SshAuthSockEnvVar contains the path of the unix file socket that the agent uses for communication with other processes
	SshAuthSockEnvVar = "SSH_AUTH_SOCK"

	// TermEnvVar defines the type of terminal the user is using
	TermEnvVar = "TERM"

	// HomeEnvVar defines home directory
	HomeEnvVar = "HOME"

	// HomePathEnvVar defines home path
	HomePathEnvVar = "HOMEPATH"

	// HomeDriveEnvVar defines home drive
	HomeDriveEnvVar = "HOMEDRIVE"

	// UserProfileEnvVar defines user profile
	UserProfileEnvVar = "USERPROFILE"

	// KubeConfigEnvVar defines the path where kubeconfig is stored
	KubeConfigEnvVar = "KUBECONFIG"

	// GithubRepositoryEnvVar defines the repository to be used
	GithubRepositoryEnvVar = "GITHUB_REPOSITORY"

	// GithubRepositoryEnvVar defines the branch to be used
	GithubRefEnvVar = "GITHUB_REF"

	// GithubRepositoryEnvVar defines the server to be used
	GithubServerURLEnvVar = "GITHUB_SERVER_URL"

	// ComposeFileEnvVar defines the compose files to use
	ComposeFileEnvVar = "COMPOSE_FILE"

	// BuildkitProgressEnvVar defines the output of buildkit
	BuildkitProgressEnvVar = "BUILDKIT_PROGRESS"

	// OktetoActionNameEnvVar defines the name of the pipeline action name
	OktetoActionNameEnvVar = "OKTETO_ACTION_NAME"

	// OktetoGitCommitPrefix prefix added to OKTETO_GIT_COMMIT when inferred by cli
	OktetoGitCommitPrefix = "dev"

	// OktetoDefaultImageTag default tag assigned to image to build
	OktetoDefaultImageTag = "okteto"
)
