package okteto

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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
			name: "graphql response",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &listEndpointsQuery{
						Response: []string{
							"https://this.is.a.test.ok",
						},
					},
				},
			},
			expected: expected{
				result: []string{
					"https://this.is.a.test.ok",
				},
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ec := endpointClient{client: tc.cfg.client}
			eps, err := ec.List(context.Background(), "ns", "label")
			assert.ErrorIs(t, err, tc.expected.err)
			assert.Equal(t, tc.expected.result, eps)
		})
	}
}
