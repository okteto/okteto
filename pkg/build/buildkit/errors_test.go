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
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func Test_GetSolveErrorMessage(t *testing.T) {
	tests := []struct {
		err      error
		expected error
		name     string
	}{
		{
			name:     "no error",
			err:      nil,
			expected: nil,
		},
		{
			name: "not authenticated",
			err:  errors.New("failed to solve: image: failed to authorize: failed to fetch anonymous token"),
			expected: oktetoErrors.UserError{
				E: fmt.Errorf("the image 'image' is not accessible or it does not exist"),
				Hint: `Please verify the name of the image to make sure it exists.
    When using private registries, make sure Okteto Registry Credentials are correctly configured.
    See more at: https://www.okteto.com/docs/admin/registry-credentials/`,
			},
		},
		{
			name: "authenticated, not authorized",
			err:  errors.New("failed to solve: image: insufficient_scope: authorization failed"),
			expected: oktetoErrors.UserError{
				E: fmt.Errorf("the image 'image' is not accessible or it does not exist"),
				Hint: `Please verify the name of the image to make sure it exists.
    When using private registries, make sure Okteto Registry Credentials are correctly configured.
    See more at: https://www.okteto.com/docs/admin/registry-credentials/`,
			},
		},
		{
			name: "buildkit service unavailable - connection refused",
			err:  errors.New("failed to solve: image: connect: connection refused"),
			expected: oktetoErrors.UserError{
				E:    fmt.Errorf("buildkit service is not available at the moment"),
				Hint: "Please try again later.",
			},
		},
		{
			name: "buildkit service unavailable - context canceled",
			err:  errors.New("failed to solve: image: context canceled"),
			expected: oktetoErrors.UserError{
				E:    fmt.Errorf("buildkit service is not available at the moment"),
				Hint: "Please try again later.",
			},
		},
		{
			name: "buildkit service unavailable - internal server error",
			err:  errors.New("failed to solve: image: 500 Internal Server Error"),
			expected: oktetoErrors.UserError{
				E:    fmt.Errorf("buildkit service is not available at the moment"),
				Hint: "Please try again later.",
			},
		},
		{
			name: "pull access denied",
			err:  errors.New("failed to solve: image: pull access denied, repository does not exist or may require authorization: server message: insufficient_scope: authorization failed"),
			expected: oktetoErrors.UserError{
				E: fmt.Errorf("the image 'image' is not accessible or it does not exist"),
				Hint: `Please verify the name of the image to make sure it exists.
    When using private registries, make sure Okteto Registry Credentials are correctly configured.
    See more at: https://www.okteto.com/docs/admin/registry-credentials/`,
			},
		},
		{
			name: "image not found at registry",
			err:  errors.New("failed to solve: image: not found"),
			expected: oktetoErrors.UserError{
				E: fmt.Errorf("the image 'image' is not accessible or it does not exist"),
				Hint: `Please verify the name of the image to make sure it exists.
    When using private registries, make sure Okteto Registry Credentials are correctly configured.
    See more at: https://www.okteto.com/docs/admin/registry-credentials/`,
			},
		},
		{
			name: "registry host not found",
			err:  errors.New("failed to solve: image: failed to do request: Head 'https://non-existing.okteto.dev/v2/xxxx/cli/manifests/debug-4': dial tcp: lookup non-existing.okteto.dev on xx.xxxx.x.xx:xx: no such host"),
			expected: oktetoErrors.UserError{
				E: fmt.Errorf("the image 'image' is not accessible or it does not exist"),
				Hint: `Please verify the name of the image to make sure it exists.
    When using private registries, make sure Okteto Registry Credentials are correctly configured.
    See more at: https://www.okteto.com/docs/admin/registry-credentials/`,
			},
		},
		{
			name: "registry host not found - airgapped with official okteto cli image",
			err:  errors.New("failed to solve: docker.io/okteto/okteto:1.2.4: failed to do request: Head 'https://non-existing.okteto.dev/v2/xxxx/cli/manifests/debug-4': dial tcp: lookup non-existing.okteto.dev on xx.xxxx.x.xx:xx: no such host"),
			expected: oktetoErrors.UserError{
				E: fmt.Errorf("the image 'docker.io/okteto/okteto:1.2.4' is not accessible or it does not exist"),
				Hint: `Please verify you have access to Docker Hub.
    If you are using an airgapped environment, make sure Okteto Remote is correctly configured in airgapped environments:
    See more at: https://www.okteto.com/docs/self-hosted/manage/air-gapped/`,
			},
		},
		{
			name: "registry host not found - airgapped with forked okteto cli image",
			err:  errors.New("failed to solve: myregistry.com/okteto/okteto:1.2.4: failed to do request: Head 'https://non-existing.okteto.dev/v2/xxxx/cli/manifests/debug-4': dial tcp: lookup non-existing.okteto.dev on xx.xxxx.x.xx:xx: no such host"),
			expected: oktetoErrors.UserError{
				E: fmt.Errorf("the image 'myregistry.com/okteto/okteto:1.2.4' is not accessible or it does not exist"),
				Hint: `Please verify you have migrated correctly to the current version for remote.
    If you are using an airgapped environment, make sure Okteto Remote is correctly configured in airgapped environments:
    See more at: https://www.okteto.com/docs/self-hosted/manage/air-gapped/`,
			},
		},
		{
			name: "okteto image not found",
			err:  errors.New("build failed: failed to solve: okteto/pipeline-runner:1.27.0-rc.14: failed to resolve source metadata for docker.io/okteto/pipeline-runner:1.27.0-rc.14: failed to do request: Head \"https://registry-1.docker.io/v2/okteto/pipeline-runner/manifests/1.27.0-rc.14\": dial tcp: lookup registry-1.docker.io on 10.96.0.10:53: no such host"),
			expected: oktetoErrors.UserError{
				E: fmt.Errorf("the image 'okteto/pipeline-runner:1.27.0-rc.14' is not accessible or it does not exist"),
				Hint: `Please verify you have access to Docker Hub.
    If you are using an airgapped environment, make sure Okteto Remote is correctly configured in airgapped environments:
    See more at: https://www.okteto.com/docs/self-hosted/manage/air-gapped/`,
			},
		},
		{
			name: "cmd error",
			err: CommandErr{
				Err:    assert.AnError,
				Stage:  "stage",
				Output: "test",
			},
			expected: CommandErr{
				Err:    assert.AnError,
				Stage:  "stage",
				Output: "test",
			},
		},
		{
			name: "default error",
			err:  assert.AnError,
			expected: oktetoErrors.UserError{
				E: assert.AnError,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GetSolveErrorMessage(tt.err)
			assert.Equal(t, tt.expected, err)
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
			res := IsRetryable(tt.err)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestExtractImageTagFromPullAccessDeniedError(t *testing.T) {
	tests := []struct {
		err      error
		expected string
	}{
		{
			err:      errors.New("pull access denied"),
			expected: "",
		},
		{
			err:      errors.New("failed to solve: registry/myimage: pull access denied, repository does not exist or may require authorizatio"),
			expected: "registry/myimage",
		},
		{
			err:      errors.New("failed to solve: myimage: pull access denied, repository does not exist or may require authorizatio"),
			expected: "myimage",
		},
		{
			err:      errors.New("failed to solve: okteto.dev/myimage: pull access denied, repository does not exist or may require authorizatio"),
			expected: "okteto.dev/myimage",
		},
		{
			err:      errors.New("failed to solve: myregistry.com/my-namespace/myimage: pull access denied, repository does not exist or may require authorizatio"),
			expected: "myregistry.com/my-namespace/myimage",
		},
		{
			err:      errors.New("failed to solve: registry/my-namespace:my-tag: pull access denied, repository does not exist or may require authorizatio"),
			expected: "registry/my-namespace:my-tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			got := extractImageFromError(tt.err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_extractImageTagFromNotFoundError(t *testing.T) {
	tests := []struct {
		err      error
		expected string
	}{
		{
			err:      errors.New("not found"),
			expected: "",
		},
		{
			err:      errors.New("failed to solve: registry/myimage: not found"),
			expected: "registry/myimage",
		},
		{
			err:      errors.New("failed to solve: registry/myimage:tag: not found"),
			expected: "registry/myimage:tag",
		},
		{
			err:      errors.New("failed to solve: myimage: not found"),
			expected: "myimage",
		},
		{
			err:      errors.New("failed to solve: okteto.dev/myimage: not found"),
			expected: "okteto.dev/myimage",
		},
		{
			err:      errors.New("failed to solve: myregistry.com/my-namespace/myimage: not found"),
			expected: "myregistry.com/my-namespace/myimage",
		},
		{
			err:      errors.New("failed to solve: registry/my-namespace:my-tag: not found"),
			expected: "registry/my-namespace:my-tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			got := extractImageFromError(tt.err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestExtractImageTagFromHostNotFound(t *testing.T) {
	tests := []struct {
		err      error
		expected string
	}{
		{
			err:      errors.New("failed to do request"),
			expected: "",
		},
		{
			err:      errors.New("failed to solve: registry/myimage: failed to do request: Head 'https://non-existing.okteto.dev/v2/xxxx/cli/manifests/debug-4': dial tcp: lookup non-existing.okteto.dev on xx.xxxx.x.xx:xx: no such host"),
			expected: "registry/myimage",
		},
		{
			err:      errors.New("failed to solve: myimage: failed to do request: Head 'https://non-existing.okteto.dev/v2/xxxx/cli/manifests/debug-4': dial tcp: lookup non-existing.okteto.dev on xx.xxxx.x.xx:xx: no such host"),
			expected: "myimage",
		},
		{
			err:      errors.New("failed to solve: okteto.dev/myimage: failed to do request: Head 'https://non-existing.okteto.dev/v2/xxxx/cli/manifests/debug-4': dial tcp: lookup non-existing.okteto.dev on xx.xxxx.x.xx:xx: no such host"),
			expected: "okteto.dev/myimage",
		},
		{
			err:      errors.New("failed to solve: myregistry.com/my-namespace/myimage: failed to do request: Head 'https://non-existing.okteto.dev/v2/xxxx/cli/manifests/debug-4': dial tcp: lookup non-existing.okteto.dev on xx.xxxx.x.xx:xx: no such host"),
			expected: "myregistry.com/my-namespace/myimage",
		},
		{
			err:      errors.New("failed to solve: registry/my-namespace:my-tag: failed to do request: Head 'https://non-existing.okteto.dev/v2/xxxx/cli/manifests/debug-4': dial tcp: lookup non-existing.okteto.dev on xx.xxxx.x.xx:xx: no such host"),
			expected: "registry/my-namespace:my-tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			got := extractImageFromError(tt.err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestExtractImageTagFromFailedToAuthorize(t *testing.T) {
	tests := []struct {
		err      error
		expected string
	}{
		{
			err:      errors.New("failed to authorize: failed to fetch anonymous token"),
			expected: "",
		},
		{
			err:      errors.New("failed to solve: registry/myimage: failed to authorize: failed to fetch anonymous token"),
			expected: "registry/myimage",
		},
		{
			err:      errors.New("failed to solve: myimage: failed to authorize: failed to fetch anonymous token"),
			expected: "myimage",
		},
		{
			err:      errors.New("failed to solve: okteto.dev/myimage: failed to authorize: failed to fetch anonymous token"),
			expected: "okteto.dev/myimage",
		},
		{
			err:      errors.New("failed to solve: myregistry.com/my-namespace/myimage: failed to authorize: failed to fetch anonymous token"),
			expected: "myregistry.com/my-namespace/myimage",
		},
		{
			err:      errors.New("failed to solve: registry/my-namespace:my-tag: failed to authorize: failed to fetch anonymous token"),
			expected: "registry/my-namespace:my-tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			got := extractImageFromError(tt.err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
