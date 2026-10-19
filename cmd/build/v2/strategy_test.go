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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBuildStrategyIsSequentialByDefault(t *testing.T) {
	t.Setenv(ParallelBuildStrategyEnvVar, "")
	t.Setenv(MaxParallelBuildsEnvVar, "")

	require.Equal(t, sequentialBuildStrategy{}, newBuildStrategy())
}

func TestNewBuildStrategyParallelWithDefaultMax(t *testing.T) {
	t.Setenv(ParallelBuildStrategyEnvVar, "true")
	t.Setenv(MaxParallelBuildsEnvVar, "")

	require.Equal(t, parallelBuildStrategy{maxParallel: 4}, newBuildStrategy())
}

func TestNewBuildStrategyParallelWithCustomMax(t *testing.T) {
	t.Setenv(ParallelBuildStrategyEnvVar, "true")
	t.Setenv(MaxParallelBuildsEnvVar, "2")

	require.Equal(t, parallelBuildStrategy{maxParallel: 2}, newBuildStrategy())
}

func TestNewBuildStrategyMaxOneIsSequential(t *testing.T) {
	t.Setenv(ParallelBuildStrategyEnvVar, "true")
	t.Setenv(MaxParallelBuildsEnvVar, "1")

	require.Equal(t, sequentialBuildStrategy{}, newBuildStrategy())
}

func TestNewBuildStrategyInvalidMaxUsesDefault(t *testing.T) {
	t.Setenv(ParallelBuildStrategyEnvVar, "true")
	t.Setenv(MaxParallelBuildsEnvVar, "many")

	require.Equal(t, parallelBuildStrategy{maxParallel: 4}, newBuildStrategy())
}

func TestBuildStrategiesMaxConcurrency(t *testing.T) {
	require.Equal(t, 1, sequentialBuildStrategy{}.maxConcurrency())
	require.Equal(t, 3, parallelBuildStrategy{maxParallel: 3}.maxConcurrency())
}
