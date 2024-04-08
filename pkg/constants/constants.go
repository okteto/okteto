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

package constants

const (
	// OktetoNameEnvVar defines if the command is running inside okteot
	OktetoNameEnvVar = "OKTETO_NAME"

	// DevLabel indicates the deployment is in dev mode
	DevLabel = "dev.okteto.com"

	// OktetoURLAnnotation indicates the okteto cluster public url
	OktetoURLAnnotation = "dev.okteto.com/url"

	// OktetoDevModeAnnotation indicates the development mode in use (sync or hybrid)
	OktetoDevModeAnnotation = "dev.okteto.com/dev-mode"

	// OktetoExtension identifies the okteto extension in kubeconfig files
	OktetoExtension = "okteto"

	// OktetoSkipConfigCredentialsUpdate prevents the kubernetes config from being updated
	// with the okteto credentials
	OktetoSkipConfigCredentialsUpdate = "OKTETO_SKIP_CONFIG_CREDENTIALS_UPDATE"

	// OktetoHomeEnvVar defines the path of okteto folder
	OktetoHomeEnvVar = "OKTETO_HOME"

	// KubeConfigEnvVar defines the path where kubeconfig is stored
	KubeConfigEnvVar = "KUBECONFIG"

	// OktetoWithinDeployCommandContextEnvVar defines if an okteto command is executed by deploy command
	OktetoWithinDeployCommandContextEnvVar = "OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT"

	// OktetoFolderEnvVar defines the path of okteto folder
	OktetoFolderEnvVar = "OKTETO_FOLDER"

	// OktetoAutodiscoveryReleaseName defines the name used for helm release when autodiscovery
	OktetoAutodiscoveryReleaseName = "OKTETO_AUTODISCOVERY_RELEASE_NAME"

	// LastUpdatedAnnotation indicates update timestamp
	LastUpdatedAnnotation = "dev.okteto.com/last-updated"

	// TimeFormat is the format to use when storing timestamps as a string
	TimeFormat = "2006-01-02T15:04:05"

	// OktetoDeployRemote defines if deployment is executed remotely
	OktetoDeployRemote = "OKTETO_DEPLOY_REMOTE"

	// OktetoForceRemote defines whether a deploy/destroy operation is to be executed remotely
	OktetoForceRemote = "OKTETO_FORCE_REMOTE"

	// OktetoTlsCertBase64EnvVar defines the TLS certificate in base64 for --remote
	OktetoTlsCertBase64EnvVar = "OKTETO_TLS_CERT_BASE64"

	// OktetoInternalServerNameEnvVar defines the internal server name for --remote
	OktetoInternalServerNameEnvVar = "INTERNAL_SERVER_NAME"

	// OktetoInvalidateCacheEnvVar defines a ramdom number to invalidate the "--remote" cache
	OktetoInvalidateCacheEnvVar = "OKTETO_INVALIDATE_CACHE"

	// OktetoDeployRemoteImage defines okteto cli image used for deploy an environment remotely
	OktetoDeployRemoteImage = "OKTETO_REMOTE_CLI_IMAGE"

	// OktetoCLIImageForRemoteTemplate defines okteto CLI image template to use for remote deployments
	OktetoCLIImageForRemoteTemplate = "okteto/okteto:%s"

	// OktetoPipelineRunnerImage defines image to use for remote deployments if empty
	OktetoPipelineRunnerImage = "okteto/pipeline-runner:1.0.2"

	// OktetoEnvFile defines the name for okteto env file
	OktetoEnvFile = "OKTETO_ENV"

	// NamespaceStatusLabel label added to namespaces to indicate its status
	NamespaceStatusLabel = "space.okteto.com/status"

	// NamespaceStatusSleeping indicates that the namespace is sleeping
	NamespaceStatusSleeping = "Sleeping"

	// DevRegistry alias url for okteto registry
	DevRegistry = "okteto.dev"

	// GlobalRegistry alias url for okteto global registry
	GlobalRegistry = "okteto.global"

	// DefaultGlobalNamespace namespace where okteto app is running
	DefaultGlobalNamespace = "okteto"

	// OktetoGitBranchEnvVar is the name of the Git branch currently being deployed.
	OktetoGitBranchEnvVar = "OKTETO_GIT_BRANCH"

	// OktetoGitCommitEnvVar is the SHA1 hash of the last commit of the branch.
	OktetoGitCommitEnvVar = "OKTETO_GIT_COMMIT"

	// OktetoNamespaceLabel is the label used to identify the namespace where the resource lives
	OktetoNamespaceLabel = "dev.okteto.com/namespace"

	// OktetoDivertWeaverDriver is the divert driver for weaver
	OktetoDivertWeaverDriver = "weaver"

	// OktetoDivertIstioDriver is the divert driver for istio
	OktetoDivertIstioDriver = "istio"

	// OktetoDivertBaggageHeader represents the baggage header
	OktetoDivertBaggageHeader = "baggage"

	// OktetoDivertHeaderName the default header name used by okteto to divert traffic
	OktetoDivertHeaderName = "okteto-divert"

	// OktetoDivertAnnotationTemplate annotation for the okteto mutation webhook to divert a virtual service
	OktetoDivertAnnotationTemplate = "divert.okteto.com/%s"

	// OktetoDeprecatedDivertAnnotationTemplate annotation for the okteto mutation webhook to divert a virtual service
	OktetoDeprecatedDivertAnnotationTemplate = "divert.okteto.com/%s-%s"

	// OktetoHybridModeFieldValue represents the hybrid mode field value
	OktetoHybridModeFieldValue = "hybrid"

	// OktetoSyncModeFieldValue represents the sync mode field value
	OktetoSyncModeFieldValue = "sync"

	// OktetoConfigMapVariablesField represents the field name related to variables seetion in config map
	OktetoConfigMapVariablesField = "variables"

	// OktetoDependencyEnvsKey the key on the conqfig map that will store OKTETO_ENV values
	OktetoDependencyEnvsKey = "dependencyEnvs"

	// EnvironmentLabelKeyPrefix represents the prefix for the preview and pipeline labels
	EnvironmentLabelKeyPrefix = "label.okteto.com"

	// OktetoDeployableEnvVar Env variable containing the piece of Okteto manifest which is deployable in remote or local deploys
	OktetoDeployableEnvVar = "OKTETO_DEPLOYABLE"

	// OktetoIsPreviewEnvVar Env variable containing a boolean indicating if the environment is a preview environment
	OktetoIsPreviewEnvVar = "OKTETO_IS_PREVIEW_ENVIRONMENT"
)
