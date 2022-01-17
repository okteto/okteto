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

const (
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

	// OktetoSkipContextTestEnvVar if set skips the context test
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

	// OktetoHomeEnvVar defines the path of okteto folder
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

	// OktetoSSHServerPortVariableEnvVar defines the remote port env var
	OktetoSSHServerPortVariableEnvVar = "OKTETO_REMOTE_PORT"

	// OktetoDefaultSSHServerPort defines the default remote port
	OktetoDefaultSSHServerPort = 2222

	// SSHAuthSockEnvVar contains the path of the unix file socket that the agent uses for communication with other processes
	SSHAuthSockEnvVar = "SSH_AUTH_SOCK"

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

	// GithubRefEnvVar defines the branch to be used
	GithubRefEnvVar = "GITHUB_REF"

	// GithubServerURLEnvVar defines the server to be used
	GithubServerURLEnvVar = "GITHUB_SERVER_URL"

	// ComposeFileEnvVar defines the compose files to use
	ComposeFileEnvVar = "COMPOSE_FILE"

	// BuildkitProgressEnvVar defines the output of buildkit
	BuildkitProgressEnvVar = "BUILDKIT_PROGRESS"
)
