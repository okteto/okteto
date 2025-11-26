// Copyright 2025 The Okteto Authors
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

	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLeastLoadedBuildKitPod_Error(t *testing.T) {
	type input struct {
		client         graphqlClientInterface
		buildRequestID string
	}
	tests := []struct {
		name          string
		input         input
		expectedError error
	}{
		{
			name: "error in graphql query",
			input: input{
				client: &fakeGraphQLClient{
					err: assert.AnError,
				},
				buildRequestID: "test-request-id",
			},
			expectedError: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := &buildkitClient{
				client: tt.input.client,
			}

			response, err := bc.GetLeastLoadedBuildKitPod(context.Background(), tt.input.buildRequestID)

			require.Error(t, err)
			require.ErrorIs(t, err, tt.expectedError)
			require.Nil(t, response)
		})
	}
}

func TestGetLeastLoadedBuildKitPod_Success(t *testing.T) {
	type input struct {
		client         graphqlClientInterface
		buildRequestID string
	}
	tests := []struct {
		name             string
		input            input
		expectedResponse *types.BuildKitPodResponse
	}{
		{
			name: "buildkit pod available",
			input: input{
				client: &fakeGraphQLClient{
					queryResult: &buildKitPodQuery{
						Response: buildKitPodResponse{
							buildKitPodAvailableFragment: buildKitPodAvailableFragment{
								PodName: graphql.String("buildkit-pod-1"),
								PodIP:   graphql.String("10.0.0.1"),
							},
							buildKitPodWaitingFragment: buildKitPodWaitingFragment{
								Reason:        graphql.String(""),
								QueuePosition: graphql.Int(0),
								TotalInQueue:  graphql.Int(0),
							},
						},
					},
				},
				buildRequestID: "test-request-id",
			},
			expectedResponse: &types.BuildKitPodResponse{
				PodName:       "buildkit-pod-1",
				PodIP:         "10.0.0.1",
				Reason:        "",
				QueuePosition: 0,
				TotalInQueue:  0,
			},
		},
		{
			name: "buildkit pod waiting in queue",
			input: input{
				client: &fakeGraphQLClient{
					queryResult: &buildKitPodQuery{
						Response: buildKitPodResponse{
							buildKitPodAvailableFragment: buildKitPodAvailableFragment{
								PodName: graphql.String(""),
								PodIP:   graphql.String(""),
							},
							buildKitPodWaitingFragment: buildKitPodWaitingFragment{
								Reason:        graphql.String("waiting for available pod"),
								QueuePosition: graphql.Int(3),
								TotalInQueue:  graphql.Int(10),
							},
						},
					},
				},
				buildRequestID: "test-request-id",
			},
			expectedResponse: &types.BuildKitPodResponse{
				PodName:       "",
				PodIP:         "",
				Reason:        "waiting for available pod",
				QueuePosition: 3,
				TotalInQueue:  10,
			},
		},
		{
			name: "buildkit pod with both available and waiting info",
			input: input{
				client: &fakeGraphQLClient{
					queryResult: &buildKitPodQuery{
						Response: buildKitPodResponse{
							buildKitPodAvailableFragment: buildKitPodAvailableFragment{
								PodName: graphql.String("buildkit-pod-2"),
								PodIP:   graphql.String("10.0.0.2"),
							},
							buildKitPodWaitingFragment: buildKitPodWaitingFragment{
								Reason:        graphql.String(""),
								QueuePosition: graphql.Int(0),
								TotalInQueue:  graphql.Int(5),
							},
						},
					},
				},
				buildRequestID: "test-request-id-2",
			},
			expectedResponse: &types.BuildKitPodResponse{
				PodName:       "buildkit-pod-2",
				PodIP:         "10.0.0.2",
				Reason:        "",
				QueuePosition: 0,
				TotalInQueue:  5,
			},
		},
		{
			name: "empty buildRequestID",
			input: input{
				client: &fakeGraphQLClient{
					queryResult: &buildKitPodQuery{
						Response: buildKitPodResponse{
							buildKitPodAvailableFragment: buildKitPodAvailableFragment{
								PodName: graphql.String("buildkit-pod-default"),
								PodIP:   graphql.String("10.0.0.3"),
							},
						},
					},
				},
				buildRequestID: "",
			},
			expectedResponse: &types.BuildKitPodResponse{
				PodName:       "buildkit-pod-default",
				PodIP:         "10.0.0.3",
				Reason:        "",
				QueuePosition: 0,
				TotalInQueue:  0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := &buildkitClient{
				client: tt.input.client,
			}

			response, err := bc.GetLeastLoadedBuildKitPod(context.Background(), tt.input.buildRequestID)

			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, tt.expectedResponse.PodName, response.PodName)
			require.Equal(t, tt.expectedResponse.PodIP, response.PodIP)
			require.Equal(t, tt.expectedResponse.Reason, response.Reason)
			require.Equal(t, tt.expectedResponse.QueuePosition, response.QueuePosition)
			require.Equal(t, tt.expectedResponse.TotalInQueue, response.TotalInQueue)
		})
	}
}

func TestNewBuildkitClient(t *testing.T) {
	tests := []struct {
		name   string
		client graphqlClientInterface
	}{
		{
			name:   "create buildkit client with fake graphql client",
			client: &fakeGraphQLClient{},
		},
		{
			name:   "create buildkit client with nil client",
			client: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := newBuildkitClient(tt.client)

			require.NotNil(t, bc)
			require.Equal(t, tt.client, bc.client)
		})
	}
}
