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

package build

import (
	"errors"
	"fmt"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
)

// getErrorMessage returns the parsed error message
func getErrorMessage(err error, tag string) error {
	if err == nil {
		return nil
	}

	imageCtrl := registry.NewImageCtrl(okteto.Config{})
	imageRegistry, imageTag := imageCtrl.GetRegistryAndRepo(tag)
	switch {
	case isLoggedIntoRegistryButDontHavePermissions(err):
		err = oktetoErrors.UserError{
			E:    fmt.Errorf("error building image '%s': You are not authorized to push image '%s'", tag, imageTag),
			Hint: fmt.Sprintf("Please log in into the registry '%s' with a user with push permissions to '%s' or use another image.", imageRegistry, imageTag),
		}
	case isNotLoggedIntoRegistry(err):
		err = oktetoErrors.UserError{
			E:    fmt.Errorf("error building image '%s': You are not authorized to push image '%s'", tag, imageTag),
			Hint: fmt.Sprintf("Log in into the registry '%s' and verify that you have permissions to push the image '%s'.", imageRegistry, imageTag),
		}
	case isBuildkitServiceUnavailable(err):
		err = oktetoErrors.UserError{
			E:    fmt.Errorf("buildkit service is not available at the moment"),
			Hint: "Please try again later.",
		}
	case isPullAccessDenied(err):
		err = oktetoErrors.UserError{
			E:    fmt.Errorf("error building image: failed to pull image '%s'. The repository is not accessible or it does not exist", imageTag),
			Hint: fmt.Sprintf("Please verify the name of the image '%s' to make sure it exists.", imageTag),
		}
	default:
		var cmdErr OktetoCommandErr
		if errors.As(err, &cmdErr) {
			return cmdErr
		}
		err = oktetoErrors.UserError{
			E: fmt.Errorf("error building image '%s': %w", tag, err),
		}
	}
	return err
}

// IsTransientError returns true if err represents a transient registry error
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	switch {
	case strings.Contains(err.Error(), "failed commit on ref") && strings.Contains(err.Error(), "500 Internal Server Error"),
		strings.Contains(err.Error(), "transport is closing"):
		return true
	case strings.Contains(err.Error(), "transport: error while dialing: dial tcp: i/o timeout"):
		return true
	case strings.Contains(err.Error(), "error reading from server: EOF"):
		return true
	case strings.Contains(err.Error(), "error while dialing: dial tcp: lookup buildkit") && strings.Contains(err.Error(), "no such host"):
		return true
	case strings.Contains(err.Error(), "failed commit on ref") && strings.Contains(err.Error(), "400 Bad Request"):
		return true
	case strings.Contains(err.Error(), "failed to do request") && strings.Contains(err.Error(), "http: server closed idle connection"):
		return true
	case strings.Contains(err.Error(), "failed to do request") && strings.Contains(err.Error(), "tls: use of closed connection"):
		return true
	case strings.Contains(err.Error(), "Canceled") && strings.Contains(err.Error(), "the client connection is closing"):
		return true
	case strings.Contains(err.Error(), "Canceled") && strings.Contains(err.Error(), "context canceled"):
		return true
	default:
		return false
	}
}

// IsLoggedIntoRegistryButDontHavePermissions returns true when the error is because the user is logged into the registry but doesn't have permissions to push the image
func isLoggedIntoRegistryButDontHavePermissions(err error) bool {
	return strings.Contains(err.Error(), "insufficient_scope: authorization failed") && !isPullAccessDenied(err)
}

// IsNotLoggedIntoRegistry returns true when the error is because the user is not logged into the registry
func isNotLoggedIntoRegistry(err error) bool {
	return strings.Contains(err.Error(), "failed to authorize: failed to fetch anonymous token") ||
		strings.Contains(err.Error(), "UNAUTHORIZED: authentication required")
}

// IsBuildkitServiceUnavailable returns true when an error is because buildkit is unavailable
func isBuildkitServiceUnavailable(err error) bool {
	return strings.Contains(err.Error(), "connect: connection refused") || strings.Contains(err.Error(), "500 Internal Server Error") || strings.Contains(err.Error(), "context canceled")
}

// IsPullAccessDenied returns true pulling an image fails (e.g: image does not exist)
func isPullAccessDenied(err error) bool {
	return strings.Contains(err.Error(), "pull access denied")
}
