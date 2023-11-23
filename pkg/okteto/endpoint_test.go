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
		result []string
		err    error
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
