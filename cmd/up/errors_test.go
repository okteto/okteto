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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isTransient(t *testing.T) {
	type input struct {
		up  *upContext
		err error
	}
	type expected struct {
		isTransient         bool
		transientRetryCount int
	}
	tests := []struct {
		input    input
		name     string
		expected expected
	}{
		{
			name: "nil error",
			input: input{
				err: nil,
				up:  &upContext{},
			},
			expected: expected{
				isTransient: false,
			},
		},
		{
			name: "transient error - early return",
			input: input{
				err: errors.New("operation time out"),
				up:  &upContext{},
			},
			expected: expected{
				isTransient: true,
			},
		},
		{
			name: "up.success false - non transient error",
			input: input{
				err: assert.AnError,
				up:  &upContext{},
			},
			expected: expected{
				isTransient: false,
			},
		},
		{
			name: "up.success false - fatal error - namespace deleted",
			input: input{
				err: errors.New("application \"test\" not found in namespace \"testns\""),
				up:  &upContext{},
			},
			expected: expected{
				isTransient: false,
			},
		},
		{
			name: "up.success true - fatal error - namespace deleted",
			input: input{
				err: errors.New("application \"test\" not found in namespace \"testns\""),
				up: &upContext{
					success: true,
				},
			},
			expected: expected{
				isTransient: false,
			},
		},
		{
			name: "up.success false - syncthing local=false didn't respond after",
			input: input{
				err: errors.New("syncthing local=false didn't respond after 1m0s"),
				up:  &upContext{},
			},
			expected: expected{
				isTransient: false,
			},
		},
		{
			name: "up.success true - syncthing local=false didn't respond after",
			input: input{
				err: errors.New("syncthing local=false didn't respond after 1m0s"),
				up: &upContext{
					success: true,
				},
			},
			expected: expected{
				isTransient:         true,
				transientRetryCount: 0,
			},
		},
		{
			name: "up.success true - retry any error",
			input: input{
				err: assert.AnError,
				up: &upContext{
					success:                      true,
					unhandledTransientMaxRetries: 5,
					unhandledTransientRetryCount: 3,
				},
			},
			expected: expected{
				isTransient:         true,
				transientRetryCount: 4,
			},
		},
		{
			name: "up.success false - max retries exceeded",
			input: input{
				err: assert.AnError,
				up: &upContext{
					success:                      true,
					unhandledTransientMaxRetries: 5,
					unhandledTransientRetryCount: 5,
				},
			},
			expected: expected{
				isTransient:         false,
				transientRetryCount: 5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isTransientErr := tt.input.up.isTransient(tt.input.err)
			assert.Equal(t, tt.expected.isTransient, isTransientErr)
			assert.Equal(t, tt.expected.transientRetryCount, tt.input.up.unhandledTransientRetryCount)
		})
	}
}

func Test_isFatalAfterSuccessError(t *testing.T) {
	tests := []struct {
		err      error
		name     string
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "recoverable error",
			err:      errors.New("some transient error"),
			expected: false,
		},
		{
			name:     "non-recoverable error - deleted namespace",
			err:      errors.New("application \"test\" not found in namespace \"testns\""),
			expected: true,
		},
		{
			name:     "non-recoverable error - deleted role binding",
			err:      errors.New("cannot get resource \"deployments\" in API group \"apps\" in the namespace \"testns\": no matching resources found"),
			expected: true,
		},
		{
			name:     "non-recoverable error - okteto down while okteto up is running",
			err:      errors.New("development container has been deactivated"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isFatalErr := isFatalAfterSuccessError(tt.err)
			assert.Equal(t, tt.expected, isFatalErr)
		})
	}
}
