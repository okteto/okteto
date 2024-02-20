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

package up

import (
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
)

// isTransient is an extension of the oktetoErrors.IsTransient, this variant is used to add transient errors dynamically
// based on the state of the upContext, in particular, if the up session was successfully started once before any retry
func (up *upContext) isTransient(err error) bool {
	if err == nil {
		return false
	}

	isTransientErr := oktetoErrors.IsTransient(err)

	// if we know the error is transient, we return early
	if isTransientErr {
		return true
	}

	if up.success {
		// if syncthing worked before (up.success == true) and it's failing now, it's worth retrying
		if strings.Contains(err.Error(), "syncthing local=false didn't respond after") {
			return true
		}
	}

	// the error is not transient, the up session has not succeeded yet, so it's not worth retrying
	return false
}
