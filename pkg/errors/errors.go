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
	ErrCommandFailed = errors.New("Command execution failed")

	// ErrNotLogged is raised when we can't get the user token
	ErrNotLogged = fmt.Errorf("please run 'okteto login [URL]' and try again")

	// ErrNotFound is raised when an object is not found
	ErrNotFound = fmt.Errorf("not found")

	// ErrInternalServerError is raised when an internal server error or similar is received
	ErrInternalServerError = fmt.Errorf("internal server error, please try again")

	// ErrQuota is returned when there aren't enough resources to enable dev mode
	ErrQuota = fmt.Errorf("Quota exceeded, please free some resources and try again")

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

	// ErrLostSyncthing is raised when we lose connectivity with syncthing
	ErrLostSyncthing = fmt.Errorf("synchronization service is disconnected")

	// ErrNotInDevMode is raised when the deployment is not in dev mode
	ErrNotInDevMode = fmt.Errorf("Deployment is not in development mode anymore")

	// ErrDevPodDeleted raised if dev pod is deleted in the middle of the "okteto up" sequence
	ErrDevPodDeleted = fmt.Errorf("development container has been removed")

	//ErrDivertNotSupported raised if the divert feature is not supported in the current cluster
	ErrDivertNotSupported = fmt.Errorf("the 'divert' field is only supported in namespaces managed by Okteto")
)

// IsNotFound returns true if err is of the type not found
func IsNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
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
		strings.Contains(err.Error(), "unexpected EOF"),
		strings.Contains(err.Error(), "TLS handshake timeout"),
		strings.Contains(err.Error(), "in the time allotted"),
		strings.Contains(err.Error(), "broken pipe"),
		strings.Contains(err.Error(), "No connection could be made"),
		strings.Contains(err.Error(), "dial tcp: operation was canceled"),
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
