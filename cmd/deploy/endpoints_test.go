package deploy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEndpoints(t *testing.T) {
	testCases := []struct {
		name           string
		expectedErr    bool
		endpointGetter *EndpointGetter
		expected       []string
	}{
		{
			name: "Get endpoints sorted",
			endpointGetter: &EndpointGetter{
				endpointControl: &fakeEndpointControl{
					endpoints: []string{
						"https://this.is.a.test.okteto",
						"https://this.is.a.test.ok",
					},
				},
			},
			expected: []string{
				"https://this.is.a.test.ok",
				"https://this.is.a.test.okteto",
			},
		},
		{
			name: "Error when retrieving endpoints",
			endpointGetter: &EndpointGetter{
				endpointControl: &fakeEndpointControl{
					err: assert.AnError,
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endpoints, err := tc.endpointGetter.getEndpoints(context.Background(), &EndpointsOptions{})
			require.Equal(t, tc.expected, endpoints)
			if tc.expectedErr {
				require.Error(t, err)
			}
		})
	}

}
