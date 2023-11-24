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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListEndpoints(t *testing.T) {
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		err    error
		result []string
	}
	testCases := []struct {
		name     string
		cfg      input
		expected expected
	}{
		{
			name: "error in graphql",
			cfg: input{
				client: &fakeGraphQLClient{
					err: assert.AnError,
				},
			},
			expected: expected{
				result: nil,
				err:    assert.AnError,
			},
		},
		{
			name: "graphql response retrieve only info related to dev env",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &SpaceQuery{
						Response: Space{
							Deployments: []Component{
								{
									Endpoints: []EndpointInfo{
										{
											Url: "https://this.is.a.test.okk",
										},
									},
									DeployedBy: "test",
								},
								{
									Endpoints: []EndpointInfo{
										{
											Url: "https://this.is.a.test.to.not.include",
										},
									},
									DeployedBy: "no-test",
								},
							},
							Statefulsets: []Component{
								{
									Endpoints: []EndpointInfo{
										{
											Url: "https://this.is.a.test.okkkk",
										},
									},
									DeployedBy: "test",
								},
							},
							Externals: []Component{
								{
									Endpoints: []EndpointInfo{
										{
											Url: "https://this.is.a.test.okkk",
										},
									},
									DeployedBy: "test",
								},
							},
							Functions: []Component{
								{
									Endpoints: []EndpointInfo{
										{
											Url: "https://this.is.a.test.ok",
										},
									},
									DeployedBy: "test",
								},
							},
						},
					},
				},
			},
			expected: expected{
				result: []string{
					"https://this.is.a.test.ok",
					"https://this.is.a.test.okk",
					"https://this.is.a.test.okkk (external)",
					"https://this.is.a.test.okkkk",
				},
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ec := endpointClient{client: tc.cfg.client}
			eps, err := ec.List(context.Background(), "ns", "test")
			assert.ErrorIs(t, err, tc.expected.err)
			require.ElementsMatch(t, eps, tc.expected.result)
		})
	}
}
