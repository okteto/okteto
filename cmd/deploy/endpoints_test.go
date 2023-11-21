package deploy

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/k8s/ingresses"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (*fakeK8sProvider) GetIngressClient() (*ingresses.Client, error) {
	return nil, nil
}

func TestGetEndpoints(t *testing.T) {
	testCases := []struct {
		name           string
		isOkteto       bool
		expectedErr    bool
		endpointGetter *EndpointGetter
		expected       []string
	}{
		{
			name: "Get endpoints sorted using okteto API",
			endpointGetter: &EndpointGetter{
				endpointControl: &fakeEndpointControl{
					endpoints: []string{
						"https://this.is.a.test.okteto",
						"https://this.is.a.test.ok",
					},
				},
			},
			isOkteto: true,
			expected: []string{
				"https://this.is.a.test.ok",
				"https://this.is.a.test.okteto",
			},
		},
		{
			name: "Error when retrieving ordered endpoints using okteto API",
			endpointGetter: &EndpointGetter{
				endpointControl: &fakeEndpointControl{
					err: assert.AnError,
				},
			},
			isOkteto:    true,
			expectedErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Namespace: "test",
						IsOkteto:  tc.isOkteto,
					},
				},
				CurrentContext: "test",
			}

			endpoints, err := tc.endpointGetter.getEndpoints(context.Background(), &EndpointsOptions{})
			require.Equal(t, tc.expected, endpoints)
			if tc.expectedErr {
				require.Error(t, err)
			}
		})
	}

}
