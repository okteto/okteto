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

package okteto

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestDeployPreview(t *testing.T) {
	type input struct {
		client    *fakeGraphQLClient
		name      string
		variables []types.Variable
	}
	type expected struct {
		response *types.PreviewResponse
		err      error
	}
	testCases := []struct {
		name     string
		input    input
		expected expected
	}{
		{
			name: "namespace validator length exceeds",
			input: input{
				name: strings.Repeat("a", 100),
			},
			expected: expected{
				response: nil,
				err: namespaceValidationError{
					object: "preview environment",
				},
			},
		},
		{
			name: "namespace validator length exceeds",
			input: input{
				name: "-",
			},
			expected: expected{
				response: nil,
				err: namespaceValidationError{
					object: "preview environment",
				},
			},
		},
		{
			name: "with variables - error",
			input: input{
				client: &fakeGraphQLClient{
					err: assert.AnError,
				},
				name: "test",
				variables: []types.Variable{
					{
						Name:  "OKTETO_ORIGIN",
						Value: "VALUE",
					},
				},
			},
			expected: expected{
				response: nil,
				err:      assert.AnError,
			},
		},
		{
			name: "with variables - no error",
			input: input{
				client: &fakeGraphQLClient{
					mutationResult: &deployPreviewMutation{
						Response: deployPreviewResponse{
							Action: actionStruct{
								Id:     "test",
								Name:   "test",
								Status: ProgressingStatus,
							},
							Preview: previewIDStruct{
								Id: "test",
							},
						},
					},
					err: nil,
				},
				name: "test",
				variables: []types.Variable{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
				},
			},
			expected: expected{
				response: &types.PreviewResponse{
					Action: &types.Action{
						ID:     "test",
						Name:   "test",
						Status: progressingStatus,
					},
					Preview: &types.Preview{
						ID: "test",
					},
				},
				err: nil,
			},
		},
		{
			name: "without variables - error",
			input: input{
				client: &fakeGraphQLClient{
					err: assert.AnError,
				},
				name:      "test",
				variables: []types.Variable{},
			},
			expected: expected{
				response: nil,
				err:      assert.AnError,
			},
		},
		{
			name: "without variables - no error",
			input: input{
				client: &fakeGraphQLClient{
					mutationResult: &deployPreviewMutation{
						Response: deployPreviewResponse{
							Action: actionStruct{
								Id:     "test",
								Name:   "test",
								Status: ProgressingStatus,
							},
							Preview: previewIDStruct{
								Id: "test",
							},
						},
					},
					err: nil,
				},
				name:      "test",
				variables: []types.Variable{},
			},
			expected: expected{
				response: &types.PreviewResponse{
					Action: &types.Action{
						ID:     "test",
						Name:   "test",
						Status: progressingStatus,
					},
					Preview: &types.Preview{
						ID: "test",
					},
				},
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pc := previewClient{
				client: tc.input.client,
			}
			response, err := pc.DeployPreview(context.Background(), tc.input.name, "", "", "", "", "", tc.input.variables)
			assert.ErrorIs(t, err, tc.expected.err)
			assert.Equal(t, tc.expected.response, response)
		})
	}
}

func TestDestroyPreview(t *testing.T) {
	type input struct {
		client *fakeGraphQLClient
		name   string
	}
	type expected struct {
		err error
	}
	testCases := []struct {
		name     string
		input    input
		expected expected
	}{
		{
			name: "no error",
			input: input{
				client: &fakeGraphQLClient{
					mutationResult: &destroyPreviewMutation{
						Response: previewIDStruct{
							Id: "test",
						},
					},
					err: nil,
				},
				name: "test",
			},
			expected: expected{
				err: nil,
			},
		},
		{
			name: "error",
			input: input{
				client: &fakeGraphQLClient{
					mutationResult: nil,
					err:            assert.AnError,
				},
				name: "test",
			},
			expected: expected{
				err: assert.AnError,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pc := previewClient{
				client: tc.input.client,
			}
			err := pc.Destroy(context.Background(), tc.input.name)
			assert.ErrorIs(t, err, tc.expected.err)
		})
	}
}

func Test_validateNamespaceName(t *testing.T) {
	tests := []struct {
		name          string
		namespace     string
		expectedError bool
		errorMessage  string
	}{
		{
			name:          "ok-namespace-starts-with-letter",
			namespace:     "argo-yournamespace",
			expectedError: false,
			errorMessage:  "",
		},
		{
			name:          "ok-namespace-starts-with-number",
			namespace:     "1-argo-yournamespace",
			expectedError: false,
			errorMessage:  "",
		},
		{
			name:          "wrong-namespace-starts-with-unsupported-character",
			namespace:     "-argo-yournamespace",
			expectedError: true,
			errorMessage:  "Malformed namespace name",
		},
		{
			name:          "wrong-namespace-unsupported-character",
			namespace:     "argo/test-yournamespace",
			expectedError: true,
			errorMessage:  "Malformed namespace name",
		},
		{
			name:          "wrong-namespace-exceeded-char-limit",
			namespace:     fmt.Sprintf("%s-yournamespace", strings.Repeat("test", 20)),
			expectedError: true,
			errorMessage:  "Exceeded number of character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNamespace(tt.namespace, "namespace")
			if err != nil && !tt.expectedError {
				t.Errorf("Expected error but no error found")
			}
			if err != nil && !tt.expectedError {
				if !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("Expected %s, but got %s", tt.errorMessage, err.Error())
				}
			}

		})
	}
}
