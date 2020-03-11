// Copyright 2020 The Okteto Authors
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

var (
	// ErrNotDevDeployment is raised when we detect that the deployment was returned to production mode
	ErrNotDevDeployment = errors.New("Deployment is no longer in developer mode")

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

	// ErrUnknownSyncError is returned when syncthing reports an unknown sync error
	ErrUnknownSyncError = fmt.Errorf("Unknown syncthing error")

	// ErrNotInCluster is returned when an unsupported command is invoked from a dev environment (e.g. okteto up)
	ErrNotInCluster = fmt.Errorf("this command is not supported from inside a pod")

	// ErrLostSyncthing is raised when we lose connectivity with syncthing
	ErrLostSyncthing = fmt.Errorf("synchronization service unresponsive")
)

// IsNotFound returns true if err is of the type not found
func IsNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}
