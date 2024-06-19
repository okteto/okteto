// Copyright 2024 The Okteto Authors
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

package args

import (
	"errors"
	"fmt"
)

var (
	// errorDevNameRequired is the error returned when the dev name is required
	errDevNameRequired = errors.New("dev name is required")

	// errorCommandRequired is the error returned when the command is required
	errCommandRequired = errors.New("command is required")

	// errNoDevContainerInDevMode is the error returned when there are no development containers in dev mode
	errNoDevContainerInDevMode = errors.New("there are no development containers in dev mode")
)

type errDevNotInManifest struct {
	devName string
}

// Error returns the error message
func (e *errDevNotInManifest) Error() string {
	return fmt.Sprintf("'%s' is not defined in your okteto manifest", e.devName)
}
