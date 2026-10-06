// Copyright 2026 The Okteto Authors
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

package v2

import (
	"context"
	"testing"

	buildTypes "github.com/okteto/okteto/cmd/build/v2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSequentialBuildStrategyBuildsInOrder(t *testing.T) {
	svcs := buildTypes.NewBuildInfos("manifest", "ns", "repo", []string{"a", "b", "c"})
	var built []string

	err := sequentialBuildStrategy{}.Build(context.Background(), svcs, func(_ context.Context, svc *buildTypes.BuildInfo) error {
		built = append(built, svc.Name())
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "c"}, built)
}

func TestSequentialBuildStrategyStopsAtFirstError(t *testing.T) {
	svcs := buildTypes.NewBuildInfos("manifest", "ns", "repo", []string{"a", "b", "c"})
	errs := map[string]error{"b": assert.AnError}
	var built []string

	err := sequentialBuildStrategy{}.Build(context.Background(), svcs, func(_ context.Context, svc *buildTypes.BuildInfo) error {
		built = append(built, svc.Name())
		return errs[svc.Name()]
	})

	require.ErrorIs(t, err, assert.AnError)
	require.Equal(t, []string{"a", "b"}, built)
}

func TestSequentialBuildStrategyWithoutServices(t *testing.T) {
	called := false

	err := sequentialBuildStrategy{}.Build(context.Background(), nil, func(context.Context, *buildTypes.BuildInfo) error {
		called = true
		return nil
	})

	require.NoError(t, err)
	require.False(t, called)
}
