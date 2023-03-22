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
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestDeployPipeline(t *testing.T) {
	type input struct {
		client    *fakeGraphQLClient
		name      string
		variables []types.Variable
	}
	type expected struct {
		response *types.GitDeployResponse
		err      error
	}
	testCases := []struct {
		name     string
		input    input
		expected expected
	}{
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
					mutationResult: &deployPipelineMutation{
						Response: deployPipelineResponse{
							Action: actionStruct{
								Id:     "test",
								Name:   "test",
								Status: ProgressingStatus,
							},
							GitDeploy: gitDeployInfoWithRepoInfo{
								Id:         "test",
								Name:       "test",
								Status:     ProgressingStatus,
								Repository: "my-repo",
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
				response: &types.GitDeployResponse{
					Action: &types.Action{
						ID:     "test",
						Name:   "test",
						Status: progressingStatus,
					},
					GitDeploy: &types.GitDeploy{
						ID:         "test",
						Name:       "test",
						Repository: "my-repo",
						Status:     progressingStatus,
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
					mutationResult: &deployPipelineMutation{
						Response: deployPipelineResponse{
							Action: actionStruct{
								Id:     "test",
								Name:   "test",
								Status: ProgressingStatus,
							},
							GitDeploy: gitDeployInfoWithRepoInfo{
								Id:         "test",
								Name:       "test",
								Status:     ProgressingStatus,
								Repository: "my-repo",
							},
						},
					},
					err: nil,
				},
				name:      "test",
				variables: []types.Variable{},
			},
			expected: expected{
				response: &types.GitDeployResponse{
					Action: &types.Action{
						ID:     "test",
						Name:   "test",
						Status: progressingStatus,
					},
					GitDeploy: &types.GitDeploy{
						ID:         "test",
						Name:       "test",
						Status:     ProgressingStatus,
						Repository: "my-repo",
					},
				},
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pc := pipelineClient{
				client: tc.input.client,
			}
			response, err := pc.Deploy(context.Background(), types.PipelineDeployOptions{
				Name:      tc.input.name,
				Variables: tc.input.variables,
			})
			assert.ErrorIs(t, err, tc.expected.err)
			assert.Equal(t, tc.expected.response, response)
		})
	}
}

func TestGetPipelineByName(t *testing.T) {
	type input struct {
		client *fakeGraphQLClient
		name   string
	}
	type expected struct {
		response *types.GitDeploy
		err      error
	}
	testCases := []struct {
		name     string
		input    input
		expected expected
	}{
		{
			name: "error",
			input: input{
				client: &fakeGraphQLClient{
					err: assert.AnError,
				},
				name: "test",
			},
			expected: expected{
				response: nil,
				err:      assert.AnError,
			},
		},
		{
			name: "not found",
			input: input{
				client: &fakeGraphQLClient{
					queryResult: &getPipelineByNameQuery{
						Response: getPipelineByNameResponse{
							GitDeploys: []gitDeployInfo{
								{
									Id:     "",
									Name:   "test1",
									Status: ProgressingStatus,
								},
								{
									Id:     "",
									Name:   "test2",
									Status: ProgressingStatus,
								},
							},
						},
					},
					err: nil,
				},
				name: "not found",
			},
			expected: expected{
				response: nil,
				err:      oktetoErrors.ErrNotFound,
			},
		},
		{
			name: "not found",
			input: input{
				client: &fakeGraphQLClient{
					queryResult: &getPipelineByNameQuery{
						Response: getPipelineByNameResponse{
							GitDeploys: []gitDeployInfo{
								{
									Id:     "",
									Name:   "test1",
									Status: ProgressingStatus,
								},
								{
									Id:     "",
									Name:   "test2",
									Status: ProgressingStatus,
								},
							},
						},
					},
					err: nil,
				},
				name: "test1",
			},
			expected: expected{
				response: &types.GitDeploy{
					ID:     "",
					Name:   "test1",
					Status: progressingStatus,
				},
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pc := pipelineClient{
				client: tc.input.client,
			}
			response, err := pc.GetByName(context.Background(), tc.input.name, "")
			assert.ErrorIs(t, err, tc.expected.err)
			assert.Equal(t, tc.expected.response, response)
		})
	}
}

func Test_getResourceFullName(t *testing.T) {
	tests := []struct {
		name    string
		kindArg string
		nameArg string
		result  string
	}{
		{
			name:    "deployment",
			kindArg: Deployment,
			nameArg: "name",
			result:  "deployment/name",
		},
		{
			name:    "statefulset",
			kindArg: StatefulSet,
			nameArg: "name",
			result:  "statefulset/name",
		},
		{
			name:    "job",
			kindArg: Job,
			nameArg: "name",
			result:  "job/name",
		},
		{
			name:    "cronjob",
			kindArg: CronJob,
			nameArg: "name",
			result:  "cronjob/name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getResourceFullName(tt.kindArg, tt.nameArg)
			if result != tt.result {
				t.Errorf("Test %s: expected %s, but got %s", tt.name, tt.result, result)
			}
		})
	}
}
