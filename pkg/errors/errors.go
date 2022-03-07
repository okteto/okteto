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

// CommandError is meant for errors displayed to the user. It can include a message and a hint
type CommandError struct {
	E      error
	Reason error
}

// Error returns the error message
func (u CommandError) Error() string {
	return fmt.Sprintf("%s: %s", u.E.Error(), strings.ToLower(u.Reason.Error()))
}

var (
	// ErrCommandFailed is raised when the command execution failed
	ErrCommandFailed = errors.New("command execution failed")

	// ErrNotLogged is raised when we can't get the user token
	ErrNotLogged = "your token is invalid. Please run 'okteto context use %s' and try again"

	// ErrCtxNotSet is raised when we don't have ctx set
	ErrCtxNotSet = fmt.Errorf("your context is not set. Please run 'okteto context' and select your context")

	// ErrNotOktetoCluster is raised when we a command is only available on an okteto cluster
	ErrNotOktetoCluster = fmt.Errorf("user is not logged in okteto cluster. Please run 'okteto context use' and select your context")

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

	// ErrManifestNotFound is raised if the okteto manifest is not found
	ErrManifestNotFound = fmt.Errorf("okteto manifest not found")

	// ErrNotInDevMode is raised when the deployment is not in dev mode
	ErrNotInDevMode = fmt.Errorf("deployment is not in development mode anymore")

	// ErrDevPodDeleted raised if dev pod is deleted in the middle of the "okteto up" sequence
	ErrDevPodDeleted = fmt.Errorf("development container has been removed")

	// ErrDivertNotSupported raised if the divert feature is not supported in the current cluster
	ErrDivertNotSupported = fmt.Errorf("the 'divert' field is only supported in namespaces managed by Okteto")

	// ContextIsNotOktetoCluster raised if the cluster connected is not managed by okteto
	ErrContextIsNotOktetoCluster = fmt.Errorf("this command is only available on Okteto Cloud or Okteto Enterprise")

	// ErrTokenFlagNeeded is raised when the command is executed from inside a pod
	ErrTokenFlagNeeded = fmt.Errorf("this command is not supported without the '--token' flag from inside a container")

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

	// ErrManifestFoundButNoDeployCommands raised when a manifest is found but no deploy commands are defined
	ErrManifestFoundButNoDeployCommands = errors.New("found okteto manifest, but no deploy commands where defined")

	// ErrUserAnsweredNoToCreateFromCompose raised when the user has selected a compose file but is trying to deploy without it
	ErrUserAnsweredNoToCreateFromCompose = fmt.Errorf("user does not want to create from compose")

	// ErrDeployHasNotDeployAnyResource raised when a deploy command has not created any resource
	ErrDeployHasNotDeployAnyResource = errors.New("it seems that you haven't deployed anything")

	// ErrDeployHasFailedCommand raised when a deploy command is executed and fails
	ErrDeployHasFailedCommand = errors.New("one of the commands in the 'deploy' section of your okteto manifests failed")
)

// IsForbidden raised if the Okteto API returns 401
func IsForbidden(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unauthorized")
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
		strings.Contains(err.Error(), "development container has been removed"):
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
