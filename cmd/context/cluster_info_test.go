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

package context

import (
	"testing"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestResolveCustomerName(t *testing.T) {
	t.Run("uses clusterinfo customer name when present", func(t *testing.T) {
		clusterInfo := &types.ClusterInfo{CustomerName: "from-clusterinfo"}
		clusterMetadata := types.ClusterMetadata{CompanyName: "from-analytics-context"}

		assert.Equal(t, "from-clusterinfo", resolveCustomerName(clusterInfo, clusterMetadata))
	})

	t.Run("falls back to analytics context when clusterinfo is nil", func(t *testing.T) {
		clusterMetadata := types.ClusterMetadata{CompanyName: "from-analytics-context"}

		assert.Equal(t, "from-analytics-context", resolveCustomerName(nil, clusterMetadata))
	})

	t.Run("falls back to analytics context when clusterinfo customer name is empty", func(t *testing.T) {
		clusterInfo := &types.ClusterInfo{CustomerName: ""}
		clusterMetadata := types.ClusterMetadata{CompanyName: "from-analytics-context"}

		assert.Equal(t, "from-analytics-context", resolveCustomerName(clusterInfo, clusterMetadata))
	})
}
