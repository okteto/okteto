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

package buildkit

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"google.golang.org/grpc/status"
)

var (
	// ErrBuildConnecionFailed is returned when the connection to buildkit fails
	ErrBuildConnecionFailed = errors.New("build connection failure")

	grpcRetryableCodes = map[uint32]interface{}{
		13: nil, // transport is closing (e.g. client buildkit pod is being deleted)
		14: nil, // unavailable (e.g. no buildkit service available)
		15: nil, // Data loss: request did not complete
	}
)

type CommandErr struct {
	Err    error
	Stage  string
	Output string
}

func (e CommandErr) Error() string {
	if e.Output == "test" {
		return fmt.Sprintf("test container '%s' failed", e.Stage)
	}
	return fmt.Sprintf("error on stage %s: %s", e.Stage, e.Err.Error())
}

var (
	oktetoRemoteCLIImage = "okteto/okteto"
	dockerhubRegistry    = "docker.io"
)

func isOktetoRemoteImage(image string) bool {
	return strings.Contains(image, oktetoRemoteCLIImage) && strings.Contains(image, dockerhubRegistry)
}

func isOktetoRemoteForkImage(image string) bool {
	return strings.Contains(image, oktetoRemoteCLIImage)
}

func isImageIsNotAccessibleErr(err error) bool {
	return isLoggedIntoRegistryButDontHavePermissions(err) ||
		isNotLoggedIntoRegistry(err) ||
		isPullAccessDenied(err) ||
		isNotFound(err) ||
		isHostNotFound(err)
}

// GetSolveErrorMessage returns the parsed error message
func GetSolveErrorMessage(err error) error {
	if err == nil {
		return nil
	}

	imageFromError := extractImageFromError(err)
	switch {
	case isBuildkitServiceUnavailable(err):
		err = oktetoErrors.UserError{
			E:    fmt.Errorf("buildkit service is not available at the moment"),
			Hint: "Please try again later.",
		}
	case isImageIsNotAccessibleErr(err):
		err = oktetoErrors.UserError{
			E: fmt.Errorf("the image '%s' is not accessible or it does not exist", imageFromError),
			Hint: `Please verify the name of the image to make sure it exists.
    When using private registries, make sure Okteto Registry Credentials are correctly configured.
    See more at: https://www.okteto.com/docs/admin/registry-credentials/`,
		}

		if isOktetoRemoteImage(imageFromError) {
			err = oktetoErrors.UserError{
				E: fmt.Errorf("the image '%s' is not accessible or it does not exist", imageFromError),
				Hint: `Please verify you have access to Docker Hub.
    If you are using an airgapped environment, make sure Okteto Remote is correctly configured in airgapped environments:
    See more at: https://www.okteto.com/docs/self-hosted/manage/air-gapped/`,
			}

		} else if isOktetoRemoteForkImage(imageFromError) {
			err = oktetoErrors.UserError{
				E: fmt.Errorf("the image '%s' is not accessible or it does not exist", imageFromError),
				Hint: `Please verify you have migrated correctly to the current version for remote.
    If you are using an airgapped environment, make sure Okteto Remote is correctly configured in airgapped environments:
    See more at: https://www.okteto.com/docs/self-hosted/manage/air-gapped/`,
			}
		}

	default:
		var cmdErr CommandErr
		if errors.As(err, &cmdErr) {
			return cmdErr
		}
		err = oktetoErrors.UserError{
			E: err,
		}
	}
	return err
}

var (
	// regexForImageFromFailedToSolveErr is the regex to extract the image from an error message
	// buildkit solve errors provide the image name between :
	regexForImageFromFailedToSolveErr = regexp.MustCompile(`: ([a-zA-Z0-9\.\/_-]+(:[a-zA-Z0-9-]+)?):`)
)

func extractImageFromError(err error) string {
	if matches := regexForImageFromFailedToSolveErr.FindStringSubmatch(err.Error()); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// IsRetryable returns true if err represents a transient registry error
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if s, ok := status.FromError(err); ok {
		if _, ok := grpcRetryableCodes[uint32(s.Code())]; ok {
			return true
		}
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

	// Transient error connection to Depot's machine
	case strings.Contains(err.Error(), "timed out connecting to machine"):
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

// IsPullAccessDenied returns true pulling an image fails because the user does not have permissions
func isPullAccessDenied(err error) bool {
	return strings.Contains(err.Error(), "pull access denied")
}

// IsNotFound returns true when the error is because the resource is not found
func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found")
}

// isHostNotFound returns true when the error is because the host is not found
func isHostNotFound(err error) bool {
	return strings.Contains(err.Error(), "failed to do request") && strings.Contains(err.Error(), "no such host")
}
