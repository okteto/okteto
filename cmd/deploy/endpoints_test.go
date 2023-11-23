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

package deploy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEndpoints(t *testing.T) {
	testCases := []struct {
		endpointGetter *EndpointGetter
		name           string
		expected       []string
		expectedErr    bool
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
