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
	oktetoLog "github.com/okteto/okteto/pkg/log"
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
		// it's important to check isFatalErr only after the first successful okteto up session, or we would retry
		// non-recoverable errors such as non-zero exit status (e.g. 'dev.[svc].command' using a cmd not in path)
		isFatalErr := isFatalAfterSuccessError(err)
		if isFatalErr {
			// non-recoverable, not worth retrying
			return false
		}

		// if syncthing worked before (up.success == true) and it's failing now, it's worth retrying
		if strings.Contains(err.Error(), "syncthing local=false didn't respond after") {
			return true
		}

		// because there might be other errors like syncthing, we retry on any error for a while
		if !isTransientErr && up.unhandledTransientRetryCount < up.unhandledTransientMaxRetries {
			oktetoLog.Debugf("handling error as transient because okteto up was successfully running before, but it's now failing with: %v (%d of %d)", err, up.unhandledTransientRetryCount, up.unhandledTransientMaxRetries)
			up.unhandledTransientRetryCount++
			return true
		}
	}

	// the error is not transient, the up session has not succeeded yet, so it's not worth retrying
	return false
}

// isFatalAfterSuccessError checks if the error is non-recoverable, and it's used to stop the retry process
func isFatalAfterSuccessError(err error) bool {
	if err == nil {
		return false
	}

	switch {

	case
		strings.Contains(err.Error(), "cannot get resource \"deployments\" in API group \"apps\" in the namespace"), // delete role binding
		strings.Contains(err.Error(), "not found in namespace"),                                                     // namespace deleted
		strings.Contains(err.Error(), "development container has been deactivated"):                                 // running `okteto down` while `okteto up` is running
		return true
	default:
		return false
	}
}
