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

	// OKtetoDeployRemote defines if deployment is executed remotely
	OKtetoDeployRemote = "OKTETO_DEPLOY_REMOTE"

	// OktetoCLIImageForRemote defines okteto CLI image to use for remote deployments
	OktetoCLIImageForRemote = "okteto/okteto:remote-deploy"

	// OktetoPipelineRunnerImage defines image to use for remote deployments if empty
	OktetoPipelineRunnerImage = "okteto/installer:1.7.6"
)
