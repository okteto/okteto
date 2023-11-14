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
	"errors"
	"fmt"
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
		labels    []string
	}
	type expected struct {
		response *types.GitDeployResponse
		err      error
	}
	testCases := []struct {
		expected expected
		name     string
		input    input
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
		{
			name: "with labels - error",
			input: input{
				client: &fakeGraphQLClient{
					err: assert.AnError,
				},
				name: "test",
				labels: []string{
					"key",
				},
			},
			expected: expected{
				response: nil,
				err:      assert.AnError,
			},
		},
		{
			name: "with labels - deprecation error",
			input: input{
				client: &fakeGraphQLClient{
					err: fmt.Errorf("Unknown argument \"labels\" on field \"deployGitRepository\" of type \"Mutation\""),
				},
				name: "test",
				labels: []string{
					"key",
				},
			},
			expected: expected{
				response: nil,
				err:      ErrDeployPipelineLabelsFeatureNotSupported,
			},
		},
		{
			name: "with labels - no error",
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
				labels: []string{
					"key",
				},
				name: "test",
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
				Labels:    tc.input.labels,
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
		expected expected
		input    input
		name     string
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
							GitDeploys: []gitDeployInfoIdNameStatus{
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
			name: "found",
			input: input{
				client: &fakeGraphQLClient{
					queryResult: &getPipelineByNameQuery{
						Response: getPipelineByNameResponse{
							GitDeploys: []gitDeployInfoIdNameStatus{
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

func TestDestroyPipeline(t *testing.T) {
	type input struct {
		client         *fakeGraphQLMultipleCallsClient
		name           string
		destroyVolumes bool
	}
	type expected struct {
		response *types.GitDeployResponse
		err      error
	}
	testCases := []struct {
		expected expected
		name     string
		input    input
	}{
		{
			name: "destroy volumes - error",
			input: input{
				client: &fakeGraphQLMultipleCallsClient{
					errs: []error{assert.AnError},
				},
				name:           "test",
				destroyVolumes: true,
			},
			expected: expected{
				err: assert.AnError,
			},
		},
		{
			name: "destroy no volumes - error",
			input: input{
				client: &fakeGraphQLMultipleCallsClient{
					errs: []error{assert.AnError},
				},
				name:           "test",
				destroyVolumes: false,
			},
			expected: expected{
				err: assert.AnError,
			},
		},
		{
			name: "destroy volumes - no error",
			input: input{
				client: &fakeGraphQLMultipleCallsClient{
					mutationResult: []interface{}{
						&destroyPipelineWithVolumesMutation{
							Response: destroyPipelineResponse{
								Action: actionStruct{
									Id:     "test",
									Name:   "test",
									Status: ProgressingStatus,
								},
								GitDeploy: gitDeployInfoWithRepoInfo{
									Id:         "test",
									Name:       "test",
									Status:     ProgressingStatus,
									Repository: "repo",
								},
							},
						},
					},
				},
				name:           "test",
				destroyVolumes: true,
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
						Repository: "repo",
						Status:     progressingStatus,
					},
				},
				err: nil,
			},
		},
		{
			name: "destroy no volumes - no error",
			input: input{
				client: &fakeGraphQLMultipleCallsClient{
					mutationResult: []interface{}{
						&destroyPipelineWithoutVolumesMutation{
							Response: destroyPipelineResponse{
								Action: actionStruct{
									Id:     "test",
									Name:   "test",
									Status: ProgressingStatus,
								},
								GitDeploy: gitDeployInfoWithRepoInfo{
									Id:         "test",
									Name:       "test",
									Status:     ProgressingStatus,
									Repository: "repo",
								},
							},
						},
					},
				},
				name:           "test",
				destroyVolumes: false,
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
						Repository: "repo",
						Status:     progressingStatus,
					},
				},
				err: nil,
			},
		},
		{
			name: "destroy volumes ->  deprecated error -> error",
			input: input{
				client: &fakeGraphQLMultipleCallsClient{
					errs: []error{
						errors.New("Cannot query field \"action\" on type \"GitDeploy\""),
						assert.AnError,
					},
				},
				name:           "test",
				destroyVolumes: true,
			},
			expected: expected{
				response: nil,
				err:      assert.AnError,
			},
		},
		{
			name: "destroy no volumes ->  deprecated error -> error",
			input: input{
				client: &fakeGraphQLMultipleCallsClient{
					errs: []error{
						errors.New("Cannot query field \"action\" on type \"GitDeploy\""),
						assert.AnError,
					},
				},
				name:           "test",
				destroyVolumes: false,
			},
			expected: expected{
				response: nil,
				err:      assert.AnError,
			},
		},
		{
			name: "destroy volumes ->  deprecated error -> error",
			input: input{
				client: &fakeGraphQLMultipleCallsClient{
					errs: []error{
						errors.New("Cannot query field \"action\" on type \"GitDeploy\""),
					},
					mutationResult: []interface{}{
						nil,
						&deprecatedDestroyPipelineWithVolumesMutation{
							Response: deprecatedDestroyPipelineResponse{
								GitDeploy: gitDeployInfoWithRepoInfo{
									Id:         "test",
									Name:       "test",
									Status:     ProgressingStatus,
									Repository: "my-repo",
								},
							},
						},
					},
				},
				name:           "test",
				destroyVolumes: true,
			},
			expected: expected{
				response: &types.GitDeployResponse{
					GitDeploy: &types.GitDeploy{
						ID:     "test",
						Status: progressingStatus,
					},
				},
				err: nil,
			},
		},
		{
			name: "destroy no volumes ->  deprecated error -> no error",
			input: input{
				client: &fakeGraphQLMultipleCallsClient{
					errs: []error{
						errors.New("Cannot query field \"action\" on type \"GitDeploy\""),
					},
					mutationResult: []interface{}{
						nil,
						&deprecatedDestroyPipelineWithoutVolumesMutation{
							Response: deprecatedDestroyPipelineResponse{
								GitDeploy: gitDeployInfoWithRepoInfo{
									Id:     "test",
									Status: ProgressingStatus,
								},
							},
						},
					},
				},
				name:           "test",
				destroyVolumes: false,
			},
			expected: expected{
				response: &types.GitDeployResponse{
					GitDeploy: &types.GitDeploy{
						ID:     "test",
						Status: progressingStatus,
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
			response, err := pc.Destroy(context.Background(), tc.input.name, "", tc.input.destroyVolumes)
			assert.ErrorIs(t, err, tc.expected.err)
			assert.Equal(t, tc.expected.response, response)
		})
	}
}

func TestGetPipelineResourcesStatus(t *testing.T) {
	type input struct {
		client     *fakeGraphQLClient
		namespace  string
		devenvName string
	}
	type expected struct {
		response map[string]string
		err      error
	}
	testCases := []struct {
		expected expected
		input    input
		name     string
	}{
		{
			name: "error",
			input: input{
				client: &fakeGraphQLClient{
					queryResult: nil,
					err:         assert.AnError,
				},
			},
			expected: expected{
				err: assert.AnError,
			},
		},
		{
			name: "no error - empty devenv - return all the resources",
			input: input{
				client: &fakeGraphQLClient{
					queryResult: &getPipelineResources{
						Response: previewResourcesStatus{
							Deployments: []resourceInfo{
								{
									ID:         "test",
									Name:       "error",
									Status:     "error",
									DeployedBy: "test",
								},
								{
									ID:         "test",
									Name:       "queued",
									Status:     "queued",
									DeployedBy: "test",
								},
								{
									ID:         "test",
									Name:       "progressing",
									Status:     "progressing",
									DeployedBy: "test",
								},
							},
							Statefulsets: []resourceInfo{
								{
									ID:         "test",
									Name:       "error",
									Status:     "error",
									DeployedBy: "test",
								},
								{
									ID:         "test",
									Name:       "queued",
									Status:     "queued",
									DeployedBy: "test",
								},
								{
									ID:         "test",
									Name:       "progressing",
									Status:     "progressing",
									DeployedBy: "test",
								},
							},
							Jobs: []resourceInfo{
								{
									ID:         "test",
									Name:       "error",
									Status:     "error",
									DeployedBy: "test",
								},
								{
									ID:         "test",
									Name:       "queued",
									Status:     "queued",
									DeployedBy: "test",
								},
								{
									ID:         "test",
									Name:       "progressing",
									Status:     "progressing",
									DeployedBy: "test",
								},
							},
							Cronjobs: []resourceInfo{
								{
									ID:         "test",
									Name:       "error",
									Status:     "error",
									DeployedBy: "test",
								},
								{
									ID:         "test",
									Name:       "queued",
									Status:     "queued",
									DeployedBy: "test",
								},
								{
									ID:         "test",
									Name:       "progressing",
									Status:     "progressing",
									DeployedBy: "test",
								},
							},
						},
					},
					err: nil,
				},
				namespace:  "test",
				devenvName: "test",
			},
			expected: expected{
				response: map[string]string{
					"deployment/error":        "error",
					"deployment/queued":       "queued",
					"deployment/progressing":  "progressing",
					"statefulset/error":       "error",
					"statefulset/queued":      "queued",
					"statefulset/progressing": "progressing",
					"job/error":               "error",
					"job/queued":              "queued",
					"job/progressing":         "progressing",
					"cronjob/error":           "error",
					"cronjob/queued":          "queued",
					"cronjob/progressing":     "progressing",
				},
				err: nil,
			},
		},
		{
			name: "no error - devenv only deploy errors",
			input: input{
				client: &fakeGraphQLClient{
					queryResult: &getPipelineResources{
						Response: previewResourcesStatus{
							Deployments: []resourceInfo{
								{
									ID:         "test",
									Name:       "error",
									Status:     "error",
									DeployedBy: "1",
								},
								{
									ID:         "test",
									Name:       "queued",
									Status:     "queued",
									DeployedBy: "2",
								},
								{
									ID:         "test",
									Name:       "progressing",
									Status:     "progressing",
									DeployedBy: "3",
								},
							},
							Statefulsets: []resourceInfo{
								{
									ID:         "test",
									Name:       "error",
									Status:     "error",
									DeployedBy: "1",
								},
								{
									ID:         "test",
									Name:       "queued",
									Status:     "queued",
									DeployedBy: "2",
								},
								{
									ID:         "test",
									Name:       "progressing",
									Status:     "progressing",
									DeployedBy: "3",
								},
							},
							Jobs: []resourceInfo{
								{
									ID:         "test",
									Name:       "error",
									Status:     "error",
									DeployedBy: "1",
								},
								{
									ID:         "test",
									Name:       "queued",
									Status:     "queued",
									DeployedBy: "2",
								},
								{
									ID:         "test",
									Name:       "progressing",
									Status:     "progressing",
									DeployedBy: "3",
								},
							},
							Cronjobs: []resourceInfo{
								{
									ID:         "test",
									Name:       "error",
									Status:     "error",
									DeployedBy: "1",
								},
								{
									ID:         "test",
									Name:       "queued",
									Status:     "queued",
									DeployedBy: "2",
								},
								{
									ID:         "test",
									Name:       "progressing",
									Status:     "progressing",
									DeployedBy: "3",
								},
							},
						},
					},
					err: nil,
				},
				namespace:  "test",
				devenvName: "1",
			},
			expected: expected{
				response: map[string]string{
					"deployment/error":  "error",
					"statefulset/error": "error",
					"job/error":         "error",
					"cronjob/error":     "error",
				},
				err: nil,
			},
		},
		{
			name: "not found -> ",
			input: input{
				client: &fakeGraphQLClient{
					queryResult: nil,
					err:         oktetoErrors.ErrNotFound,
				},
				namespace:  "test",
				devenvName: "non-existing",
			},
			expected: expected{
				response: nil,
				err:      errURLNotSet,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pc := pipelineClient{
				client: tc.input.client,
			}
			response, err := pc.GetResourcesStatus(context.Background(), tc.input.devenvName, tc.input.namespace)
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
