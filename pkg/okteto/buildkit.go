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
	"errors"
	"fmt"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

// ErrIncompatibleBackend is returned when the backend does not support the buildkit pod endpoint
var ErrIncompatibleBackend = errors.New("backend does not support buildkit pod management")

// buildKitPodQuery represents the query to get the least loaded buildkit pod
type buildKitPodQuery struct {
	Response buildKitPodResponse `graphql:"getLeastLoadedBuildKitPod(buildRequestID: $buildRequestID)"`
}

// buildKitPodResponse represents the response from the getLeastLoadedBuildKitPod query
type buildKitPodResponse struct {
	buildKitPodAvailableFragment `graphql:"... on BuildKitPodAvailable"`
	buildKitPodWaitingFragment   `graphql:"... on BuildKitPodWaiting"`
}

// buildKitPodAvailableFragment represents a buildkit pod that is ready to use
type buildKitPodAvailableFragment struct {
	PodName graphql.String `graphql:"podName"`
	PodIP   graphql.String `graphql:"podIP"`
}

// buildKitPodWaitingFragment represents a buildkit request that is waiting in queue
type buildKitPodWaitingFragment struct {
	Reason        graphql.String
	QueuePosition graphql.Int
	TotalInQueue  graphql.Int
}

type buildkitClient struct {
	client graphqlClientInterface
}

func newBuildkitClient(client graphqlClientInterface) *buildkitClient {
	return &buildkitClient{
		client: client,
	}
}

// GetLeastLoadedBuildKitPod retrieves the least loaded buildkit pod from the API
func (c *buildkitClient) GetLeastLoadedBuildKitPod(ctx context.Context, buildRequestID string) (*types.BuildKitPodResponse, error) {
	oktetoLog.Infof("getting least loaded buildkit pod for build request: %s", buildRequestID)
	var queryStruct buildKitPodQuery

	variables := map[string]interface{}{
		"buildRequestID": graphql.String(buildRequestID),
	}

	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		// Detect if the backend doesn't support the buildkit pod endpoint
		// GraphQL returns "Cannot query field" error when the field doesn't exist in the schema
		errStr := err.Error()
		if strings.Contains(errStr, "Cannot query field") && strings.Contains(errStr, "getLeastLoadedBuildKitPod") {
			return nil, ErrIncompatibleBackend
		}
		return nil, fmt.Errorf("failed to get least loaded buildkit pod: %w", err)
	}

	// Convert internal types to public types
	return &types.BuildKitPodResponse{
		PodName:       string(queryStruct.Response.buildKitPodAvailableFragment.PodName),
		PodIP:         string(queryStruct.Response.buildKitPodAvailableFragment.PodIP),
		Reason:        string(queryStruct.Response.buildKitPodWaitingFragment.Reason),
		QueuePosition: int(queryStruct.Response.buildKitPodWaitingFragment.QueuePosition),
		TotalInQueue:  int(queryStruct.Response.buildKitPodWaitingFragment.TotalInQueue),
	}, nil
}
