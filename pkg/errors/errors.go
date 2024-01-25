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

package errors

import (
	"errors"
	"fmt"
	"strings"
)

// UserError is meant for errors displayed to the user. It can include a message and a hint
type UserError struct {
	E    error
	Hint string
}

// Error returns the error message
func (u UserError) Error() string {
	return u.E.Error()
}
func (e UserError) Unwrap() error {
	return e.E
}

// CommandError is meant for errors displayed to the user. It can include a message and a hint
type CommandError struct {
	E      error
	Reason error
}

// Error returns the error message
func (u CommandError) Error() string {
	return fmt.Sprintf("%s: %s", u.E.Error(), strings.ToLower(u.Reason.Error()))
}

const (
	// InvalidDockerfile text error
	InvalidDockerfile = "invalid Dockerfile"
)

// NotLoggedError is raised when the user is not logged in okteto
type NotLoggedError struct {
	Context string
}

// Error returns the error message
func (e NotLoggedError) Error() string {
	return fmt.Sprintf(ErrNotLogged, e.Context)
}

func (NotLoggedError) Unwrap() error {
	return ErrNotLoggedMsg
}

var (
	// ErrCommandFailed is raised when the command execution failed
	ErrCommandFailed = errors.New("command execution failed")

	// ErrNotLoggedMsg is raised when the user is not logged in okteto
	ErrNotLoggedMsg = errors.New("user is not logged in okteto")

	// ErrNotLogged is raised when we can't get the user token
	ErrNotLogged = "your token is invalid. Please run 'okteto context use %s' and try again"

	// ErrCtxNotSet is raised when we don't have ctx set
	ErrCtxNotSet = fmt.Errorf("your context is not set. Please run 'okteto context' and select your context")

	// ErrNotOktetoCluster is raised when a command is only available on an okteto cluster
	ErrNotOktetoCluster = fmt.Errorf("user is not logged in okteto context. Please run 'okteto context use' and select your context")

	// ErrNotFound is raised when an object is not found
	ErrNotFound = fmt.Errorf("not found")

	// ErrInternalServerError is raised when an internal server error or similar is received
	ErrInternalServerError = fmt.Errorf("internal server error, please try again")

	// ErrQuota is returned when there aren't enough resources to enable dev mode
	ErrQuota = fmt.Errorf("quota exceeded, please free some resources and try again")

	// ErrSSHConnectError is returned when okteto cannot connect to ssh
	ErrSSHConnectError = fmt.Errorf("ssh start error")

	// ErrNotInDevContainer is returned when an unsupported command is invoked from a dev container (e.g. okteto up)
	ErrNotInDevContainer = fmt.Errorf("'OKTETO_NAME' environment variable is defined. This command is not supported from inside a development container")

	// ErrUnknownSyncError is returned when syncthing reports an unknown sync error
	ErrUnknownSyncError = fmt.Errorf("unknown syncthing error")

	// ErrNeedsResetSyncError is returned when syncthing reports an inconsistent database state that needs to be reset
	ErrNeedsResetSyncError = fmt.Errorf("needs syncthing reset error")

	// ErrInsufficientSpace is raised when syncthing fails with no space available
	ErrInsufficientSpace = fmt.Errorf("there isn't enough disk space available to synchronize your files")

	// ErrBusySyncthing is raised when syncthing is busy
	ErrBusySyncthing = fmt.Errorf("synchronization service is unresponsive")

	// ErrDeleteToApp is raised when the app is deleted while running "okteto up"
	ErrDeleteToApp = fmt.Errorf("application has been deleted. Run 'okteto down -v' to delete the resources created by your development container")

	// ErrApplyToApp is raised when the app is modified while running "okteto up"
	ErrApplyToApp = fmt.Errorf("application has been modified")

	// ErrLostSyncthing is raised when we lose connectivity with syncthing
	ErrLostSyncthing = fmt.Errorf("synchronization service is disconnected")

	// ErrNotInDevMode is raised when the deployment is not in dev mode
	ErrNotInDevMode = fmt.Errorf("deployment is not in development mode anymore")

	// ErrDevPodDeleted raised if dev pod is deleted in the middle of the "okteto up" sequence
	ErrDevPodDeleted = fmt.Errorf("development container has been removed")

	// ErrDivertNotSupported raised if the divert feature is not supported in the current cluster
	ErrDivertNotSupported = fmt.Errorf("the 'divert' field is only supported in contexts that have Okteto installed")

	// ErrContextIsNotOktetoCluster raised if the cluster connected is not managed by okteto
	ErrContextIsNotOktetoCluster = fmt.Errorf("this command is only available in contexts where Okteto is installed.\n Follow this link to know more about configuring Okteto at your context: https://www.okteto.com/docs/get-started/install-okteto-cli/#configuring-okteto-cli-with-okteto")

	// ErrTokenFlagNeeded is raised when the command is executed from inside a pod from a ctx command
	ErrTokenFlagNeeded = fmt.Errorf("this command is not supported without the '--token' flag from inside a container")

	// ErrTokenEnvVarNeeded is raised when the command is executed from inside a pod from a non ctx command
	ErrTokenEnvVarNeeded = fmt.Errorf("the 'OKTETO_TOKEN' environment variable is required when running this command from within a container")

	// ErrNamespaceNotFound is raised when the namespace is not found on an okteto instance
	ErrNamespaceNotFound = "namespace '%s' not found. Please verify that the namespace exists and that you have access to it"

	// ErrKubernetesContextNotFound is raised when the kubernetes context is not found in kubeconfig
	ErrKubernetesContextNotFound = "context '%s' not found in '%s'"

	// ErrNamespaceNotMatching is raised when the namespace arg doesn't match the manifest namespace
	ErrNamespaceNotMatching = fmt.Errorf("the namespace in the okteto manifest doesn't match your namespace argument")

	// ErrContextNotMatching is raised when the context arg doesn't match the manifest context
	ErrContextNotMatching = fmt.Errorf("the context in the okteto manifest doesn't match your context argument")

	// ErrCorruptedOktetoContexts raised when the okteto context store is corrupted
	ErrCorruptedOktetoContexts = "okteto context store is corrupted. Delete the folder '%s' and try again"

	// ErrIntSig raised if the we get an interrupt signal in the middle of a command
	ErrIntSig = fmt.Errorf("interrupt signal received")

	// ErrKubernetesLongTimeToCreateDevContainer raised when the creation of the dev container times out
	ErrKubernetesLongTimeToCreateDevContainer = fmt.Errorf("kubernetes is taking too long to start your development container. Please check for errors and try again")

	// ErrNoServicesinOktetoManifest raised when no services are defined in the okteto manifest
	ErrNoServicesinOktetoManifest = fmt.Errorf("'okteto restart' is only supported when using the field 'services'")

	// ErrManifestFoundButNoDeployAndDependenciesCommands raised when a manifest is found but no deploy or dependencies commands are defined
	ErrManifestFoundButNoDeployAndDependenciesCommands = errors.New("found okteto manifest, but no deploy or dependencies commands were defined")

	// ErrDeployCantDeploySvcsIfNotCompose raised when a manifest is found but no compose info is detected and args are passed to deploy command
	ErrDeployCantDeploySvcsIfNotCompose = errors.New("services args are can only be used while trying to deploy a compose")

	// ErrUserAnsweredNoToCreateFromCompose raised when the user has selected a compose file but is trying to deploy without it
	ErrUserAnsweredNoToCreateFromCompose = fmt.Errorf("user does not want to create from compose")

	// ErrDeployHasFailedCommand raised when a deploy command is executed and fails
	ErrDeployHasFailedCommand = errors.New("one of the commands in the 'deploy' section of your okteto manifests failed")

	// ErrGitHubNotVerifiedEmail is raised when github login has not a verified email
	ErrGitHubNotVerifiedEmail = errors.New("github-not-verified-email")

	// ErrBuiltInOktetoEnvVarSetFromCMD is raised when user tries to set an okteto built-in environment variable
	ErrBuiltInOktetoEnvVarSetFromCMD = errors.New("okteto built-in environment variable cannot be set from 'okteto up' command")

	// ErrNoServicesToBuildDefined is raised when there are no services to build and buildV2 is called
	ErrNoServicesToBuildDefined = errors.New("no services to build defined")

	// ErrNoFlagAllowedOnSingleImageBuild is raised when the user tries to build a single image with flags
	ErrNoFlagAllowedOnSingleImageBuild = errors.New("flags only allowed when building a single image with `okteto build [NAME]`")

	// ErrManifestNoDevSection is raised when the manifest doesn't have a dev section and the user tries to access it
	ErrManifestNoDevSection = errors.New("okteto manifest has no 'dev' section. Configure it with 'okteto init'")

	// ErrDevContainerNotExists is raised when the dev container doesn't exist on dev section
	ErrDevContainerNotExists = "development container '%s' doesn't exist"

	// ErrInvalidManifest is raised when cannot unmarshal manifest properly
	ErrInvalidManifest = errors.New("your okteto manifest is not valid, please check the following errors")

	// ErrServiceEmpty is raised whenever service is empty in docker-compose
	ErrServiceEmpty = errors.New("service cannot be empty")

	// ErrEmptyManifest is raised when cannot detected content to read in manifest
	ErrEmptyManifest = errors.New("no content detected for okteto.yml file")

	// ErrPortAlreadyAllocated is raised when port is allocated by other process
	ErrPortAlreadyAllocated = errors.New("port is already allocated")

	// ErrNotManifestContentDetected is raised when cannot load any field accepted by okteto manifest doc
	ErrNotManifestContentDetected = errors.New("couldn't detect okteto manifest content")

	// ErrCouldNotInferAnyManifest is raised when we can't detect any manifest to load
	ErrCouldNotInferAnyManifest = errors.New("couldn't detect any manifest (okteto manifest, pipeline, compose, helm chart, k8s manifest)")

	// ErrX509Hint should be included within a UserError.Hint when IsX509() return true
	ErrX509Hint = "Add the flag '--insecure-skip-tls-verify' to skip certificate verification.\n    Follow this link to know more about configuring your own certificates with Okteto:\n    https://www.okteto.com/docs/self-hosted/install/certificates/"

	// ErrTimeout is raised when an operation has timed out
	ErrTimeout = fmt.Errorf("operation timed out")

	// ErrInvalidLicense is the error returned to the user when a trial is invalid.
	// This can be either an expired trial license or no license at all
	ErrInvalidLicense = errors.New("Your license is invalid")

	// ErrTokenExpired is raised when token used for API auth is expired
	ErrTokenExpired = errors.New("your token has expired")
)

// IsAlreadyExists raised if the Kubernetes API returns AlreadyExists
func IsAlreadyExists(err error) bool {
	return err != nil && strings.Contains(err.Error(), "already exists")
}

// IsForbidden raised if the Okteto API returns 401
func IsForbidden(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unauthorized")
}

// IsX509 raised if the Okteto API returns an error which contains x509
func IsX509(err error) bool {
	return err != nil && strings.Contains(err.Error(), "x509")
}

// IsNotFound returns true if err is of the type not found
func IsNotFound(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "doesn't exist") || strings.Contains(err.Error(), "not-found"))
}

// IsNotExist returns true if err is of the type does not exist
func IsNotExist(err error) bool {
	if err == nil {
		return false
	}

	switch {
	case strings.Contains(err.Error(), "does not exist"),
		strings.Contains(err.Error(), "doesn't exist"):
		return true
	default:
		return false
	}
}

// IsTransient returns true if err represents a transient error
func IsTransient(err error) bool {
	if err == nil {
		return false
	}

	switch {
	case strings.Contains(err.Error(), "operation time out"),
		strings.Contains(err.Error(), "operation timed out"),
		strings.Contains(err.Error(), "i/o timeout"),
		strings.Contains(err.Error(), "unknown (get events)"),
		strings.Contains(err.Error(), "Client.Timeout exceeded while awaiting headers"),
		strings.Contains(err.Error(), "can't assign requested address"),
		strings.Contains(err.Error(), "command exited without exit status or exit signal"),
		strings.Contains(err.Error(), "connection refused"),
		strings.Contains(err.Error(), "connection reset by peer"),
		strings.Contains(err.Error(), "client connection lost"),
		strings.Contains(err.Error(), "nodename nor servname provided, or not known"),
		strings.Contains(err.Error(), "no route to host"),
		strings.Contains(err.Error(), "unexpected EOF"),
		strings.Contains(err.Error(), "TLS handshake timeout"),
		strings.Contains(err.Error(), "in the time allotted"),
		strings.Contains(err.Error(), "broken pipe"),
		strings.Contains(err.Error(), "No connection could be made"),
		strings.Contains(err.Error(), "operation was canceled"),
		strings.Contains(err.Error(), "network is unreachable"),
		strings.Contains(err.Error(), "development container has been removed"),
		strings.Contains(err.Error(), "unexpected packet in response to channel open"),
		strings.Contains(err.Error(), "closing remote connection: EOF"),
		strings.Contains(err.Error(), "request for pseudo terminal failed: eof"),
		strings.Contains(err.Error(), "unable to upgrade connection"),
		strings.Contains(err.Error(), "command execution failed: eof"):
		return true
	default:
		return false
	}
}

// IsClosedNetwork returns true if the error is caused by a closed network connection
func IsClosedNetwork(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "use of closed network connection")
}

func IsErrGitHubNotVerifiedEmail(err error) bool {
	return err.Error() == ErrGitHubNotVerifiedEmail.Error()
}
