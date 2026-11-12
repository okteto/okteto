// Copyright 2026 The Okteto Authors
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

package v2

import (
	"errors"
	"io/fs"
	"strings"

	"github.com/okteto/okteto/pkg/build/buildkit"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
)

// Categories of the image_build analytics event's error_reason property.
// The raw error is never sent: it can hold credentials (e.g. tokens in a
// registry or git URL) or local paths.
const (
	buildErrorQueueTimeout      = "queue_timeout"
	buildErrorConnectionFailure = "connection_failure"
	buildErrorRegistryError     = "registry_error"
	buildErrorImageNotFound     = "image_not_found"
	buildErrorDockerfileMissing = "dockerfile_missing"
	buildErrorDockerfileFailed  = "dockerfile_failed"
	buildErrorSecretError       = "secret_error"
	buildErrorUnknown           = "unknown"
)

// Fragments of the error messages built by the build code. Most build errors are
// rewritten into UserErrors with a new message (see buildkit.GetSolveErrorMessage
// and the BuildKit connectors), so the message is the only thing left to match on.
const (
	secretErrorMsg            = "failed to process build secrets"
	queueTimeoutMsg           = "waiting in the Okteto Build queue timed out"
	buildkitUnavailableMsg    = "buildkit service is not available"
	buildkitWaitTimeoutMsg    = "buildkit service not available for"
	assignBuildkitPodMsg      = "failed to assign buildkit pod"
	leastLoadedBuildkitPodMsg = "could not get least loaded buildkit pod"
	portForwardCreationMsg    = "port forward creation to BuildKit has failed"
	registryServerErrorMsg    = "the registry returned '"
	registryDigestErrorMsg    = "error accessing image at registry"
	imageNotAccessibleMsg     = "is not accessible or it does not exist"
	// BuildKit's own errors, returned as they are by GetSolveErrorMessage:
	// a failed RUN step, a COPY/ADD of a missing file and a Dockerfile syntax error
	runStepFailedMsg     = "did not complete successfully"
	copyChecksumErrorMsg = "failed to calculate checksum of ref"
	dockerfileParseMsg   = "dockerfile parse error"
)

// imageBuildErrorReason maps a service build error to a bounded category for
// the image_build analytics event. The order matters: the secrets error wraps a
// file-not-found error, which would otherwise read as a missing Dockerfile, and
// the Dockerfile step errors hold the user's command, which must not be matched
// against the connection and registry messages.
func imageBuildErrorReason(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, secretErrorMsg):
		return buildErrorSecretError
	case errors.As(err, &buildkit.CommandErr{}),
		strings.Contains(msg, runStepFailedMsg),
		strings.Contains(msg, copyChecksumErrorMsg),
		strings.Contains(msg, dockerfileParseMsg):
		return buildErrorDockerfileFailed
	case strings.Contains(msg, queueTimeoutMsg):
		return buildErrorQueueTimeout
	case errors.Is(err, buildkit.ErrBuildConnecionFailed),
		strings.Contains(msg, buildkitUnavailableMsg),
		strings.Contains(msg, buildkitWaitTimeoutMsg),
		strings.Contains(msg, assignBuildkitPodMsg),
		strings.Contains(msg, leastLoadedBuildkitPodMsg),
		strings.Contains(msg, portForwardCreationMsg):
		return buildErrorConnectionFailure
	case strings.Contains(msg, registryServerErrorMsg),
		strings.Contains(msg, registryDigestErrorMsg):
		return buildErrorRegistryError
	case strings.Contains(msg, imageNotAccessibleMsg):
		return buildErrorImageNotFound
	// basic.Builder reports a missing Dockerfile with the manifest errors
	case errors.Is(err, oktetoErrors.ErrManifestPathNotFound),
		errors.Is(err, oktetoErrors.ErrManifestPathIsDir),
		errors.Is(err, fs.ErrNotExist):
		return buildErrorDockerfileMissing
	default:
		return buildErrorUnknown
	}
}
