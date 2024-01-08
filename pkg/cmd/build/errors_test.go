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
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func Test_getErrorMessage(t *testing.T) {
	imageTag := "example/image:latest"

	tests := []struct {
		err      error
		expected error
		name     string
		tag      string
	}{
		{
			name:     "no error",
			err:      nil,
			expected: nil,
		},
		{
			name: "logged in but no permissions",
			err:  errors.New("insufficient_scope: authorization failed"),
			tag:  imageTag,
			expected: oktetoErrors.UserError{
				E:    fmt.Errorf("error building image '%s': You are not authorized to push image '%s'", imageTag, imageTag),
				Hint: fmt.Sprintf("Please log in into the registry '%s' with a user with push permissions to '%s' or use another image.", "docker.io", imageTag),
			},
		},
		{
			name: "not logged in",
			err:  errors.New("failed to authorize: failed to fetch anonymous token"),
			tag:  imageTag,
			expected: oktetoErrors.UserError{
				E:    fmt.Errorf("error building image '%s': You are not authorized to push image '%s'", imageTag, imageTag),
				Hint: fmt.Sprintf("Log in into the registry '%s' and verify that you have permissions to push the image '%s'.", "docker.io", imageTag),
			},
		},
		{
			name: "buildkit service unavailable",
			err:  errors.New("connect: connection refused"),
			tag:  imageTag,
			expected: oktetoErrors.UserError{
				E:    fmt.Errorf("buildkit service is not available at the moment"),
				Hint: "Please try again later.",
			},
		},
		{
			name: "buildkit service unavailable",
			err:  errors.New("connect: connection refused"),
			tag:  imageTag,
			expected: oktetoErrors.UserError{
				E:    fmt.Errorf("buildkit service is not available at the moment"),
				Hint: "Please try again later.",
			},
		},
		{
			name: "buildkit service unavailable",
			err:  errors.New("500 Internal Server Error"),
			tag:  imageTag,
			expected: oktetoErrors.UserError{
				E:    fmt.Errorf("buildkit service is not available at the moment"),
				Hint: "Please try again later.",
			},
		},
		{
			name: "buildkit service unavailable",
			err:  errors.New("context canceled"),
			tag:  imageTag,
			expected: oktetoErrors.UserError{
				E:    fmt.Errorf("buildkit service is not available at the moment"),
				Hint: "Please try again later.",
			},
		},
		{
			name: "pull access denied",
			err:  errors.New("pull access denied, repository does not exist or may require authorization: server message: insufficient_scope: authorization failed"),
			tag:  imageTag,
			expected: oktetoErrors.UserError{
				E:    fmt.Errorf("error building image: failed to pull image '%s'. The repository is not accessible or it does not exist", imageTag),
				Hint: fmt.Sprintf("Please verify the name of the image '%s' to make sure it exists.", imageTag),
			},
		},
		{
			name: "not handled",
			err:  assert.AnError,
			tag:  imageTag,
			expected: oktetoErrors.UserError{
				E: fmt.Errorf("error building image '%s': %s", imageTag, assert.AnError),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := getErrorMessage(tt.err, tt.tag)
			if tt.expected == nil {
				assert.Nil(t, err)
			} else {
				assert.ErrorContains(t, tt.expected, err.Error())
			}
		})
	}
}

func Test_isTransientError(t *testing.T) {
	tests := []struct {
		err      error
		name     string
		expected bool
	}{
		{
			name:     "contains 'failed commit on ref' and '500 Internal Server Error'",
			err:      fmt.Errorf("failed commit on ref: 500 Internal Server Error"),
			expected: true,
		},
		{
			name:     "contains 'transport is closing'",
			err:      fmt.Errorf("transport is closing"),
			expected: true,
		},
		{
			name:     "contains 'transport: error while dialing: dial tcp: i/o timeout'",
			err:      fmt.Errorf("transport: error while dialing: dial tcp: i/o timeout"),
			expected: true,
		},
		{
			name:     "contains 'error reading from server: EOF'",
			err:      fmt.Errorf("error reading from server: EOF"),
			expected: true,
		},
		{
			name:     "contains 'error while dialing: dial tcp: lookup buildkit' and 'no such host'",
			err:      fmt.Errorf("error while dialing: dial tcp: lookup buildkit: no such host"),
			expected: true,
		},
		{
			name:     "contains 'failed commit on ref' and '400 Bad Request'",
			err:      fmt.Errorf("failed commit on ref: 400 Bad Request"),
			expected: true,
		},
		{
			name:     "contains 'failed to do request' and 'http: server closed idle connection'",
			err:      fmt.Errorf("failed to do request: http: server closed idle connection"),
			expected: true,
		},
		{
			name:     "contains 'failed to do request' and 'tls: use of closed connection'",
			err:      fmt.Errorf("failed to do request: tls: use of closed connection"),
			expected: true,
		},
		{
			name:     "contains 'Canceled' and 'the client connection is closing'",
			err:      fmt.Errorf("Canceled: the client connection is closing"),
			expected: true,
		},
		{
			name:     "contains 'Canceled' and 'context canceled'",
			err:      fmt.Errorf("Canceled: context canceled"),
			expected: true,
		},
		{
			name:     "contains other error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := isTransientError(tt.err)
			assert.Equal(t, tt.expected, res)
		})
	}
}
