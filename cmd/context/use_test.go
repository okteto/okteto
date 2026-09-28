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
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/build/buildkit"
	"github.com/okteto/okteto/pkg/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_setSecrets(t *testing.T) {
	key := "key"
	expectedValue := "value"
	var tests = []struct {
		envs    map[string]string
		name    string
		secrets []env.Var
	}{
		{
			name: "create new env var from secret",
			secrets: []env.Var{
				{
					Name:  key,
					Value: expectedValue,
				},
			},
			envs: map[string]string{},
		},
		{
			name: "not overwrite env var from secret",
			secrets: []env.Var{
				{
					Name:  key,
					Value: "random-value",
				},
			},
			envs: map[string]string{
				key: expectedValue,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}
			exportPlatformVariablesToEnv(tt.secrets)
			assert.Equal(t, expectedValue, os.Getenv(key))
		})
	}
}

// clearEnv unsets the given env vars for the duration of the test, restoring
// their original values (and removing any value written by the code under test) on cleanup
func clearEnv(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		t.Setenv(name, "")
		require.NoError(t, os.Unsetenv(name))
	}
}

func Test_exportPlatformVariablesToEnv_LocalAlphaPreventsPlatformBothNames(t *testing.T) {
	clearEnv(t, buildkit.BuildCompressionEnvVar, buildkit.AlphaBuildCompressionEnvVar)
	t.Setenv(buildkit.AlphaBuildCompressionEnvVar, "zstd")

	exportPlatformVariablesToEnv([]env.Var{
		{Name: buildkit.BuildCompressionEnvVar, Value: "gzip"},
		{Name: buildkit.AlphaBuildCompressionEnvVar, Value: "gzip"},
	})

	require.Equal(t, "zstd", os.Getenv(buildkit.AlphaBuildCompressionEnvVar))
	_, exists := os.LookupEnv(buildkit.BuildCompressionEnvVar)
	require.False(t, exists)
}

func Test_exportPlatformVariablesToEnv_LocalEmptyNewPreventsPlatformBothNames(t *testing.T) {
	clearEnv(t, buildkit.BuildCompressionEnvVar, buildkit.AlphaBuildCompressionEnvVar)
	t.Setenv(buildkit.BuildCompressionEnvVar, "")

	exportPlatformVariablesToEnv([]env.Var{
		{Name: buildkit.BuildCompressionEnvVar, Value: "zstd"},
		{Name: buildkit.AlphaBuildCompressionEnvVar, Value: "zstd"},
	})

	value, exists := os.LookupEnv(buildkit.BuildCompressionEnvVar)
	require.True(t, exists)
	require.Equal(t, "", value)
	_, exists = os.LookupEnv(buildkit.AlphaBuildCompressionEnvVar)
	require.False(t, exists)
}

func Test_exportPlatformVariablesToEnv_NoLocalPlatformSendsBothNames(t *testing.T) {
	clearEnv(t,
		buildkit.BuildCompressionEnvVar, buildkit.AlphaBuildCompressionEnvVar,
		buildkit.BuildCompressionLevelEnvVar, buildkit.AlphaBuildCompressionLevelEnvVar,
		buildkit.BuildForceCompressionEnvVar, buildkit.AlphaBuildForceCompressionEnvVar,
	)

	exportPlatformVariablesToEnv([]env.Var{
		{Name: buildkit.BuildCompressionEnvVar, Value: "zstd"},
		{Name: buildkit.AlphaBuildCompressionEnvVar, Value: "zstd"},
		{Name: buildkit.BuildCompressionLevelEnvVar, Value: "3"},
		{Name: buildkit.AlphaBuildCompressionLevelEnvVar, Value: "3"},
		{Name: buildkit.BuildForceCompressionEnvVar, Value: "true"},
		{Name: buildkit.AlphaBuildForceCompressionEnvVar, Value: "true"},
	})

	require.Equal(t, "zstd", os.Getenv(buildkit.BuildCompressionEnvVar))
	require.Equal(t, "zstd", os.Getenv(buildkit.AlphaBuildCompressionEnvVar))
	require.Equal(t, "3", os.Getenv(buildkit.BuildCompressionLevelEnvVar))
	require.Equal(t, "3", os.Getenv(buildkit.AlphaBuildCompressionLevelEnvVar))
	require.Equal(t, "true", os.Getenv(buildkit.BuildForceCompressionEnvVar))
	require.Equal(t, "true", os.Getenv(buildkit.AlphaBuildForceCompressionEnvVar))
}

func Test_exportPlatformVariablesToEnv_NoLocalPlatformSendsBothNamesAlphaFirst(t *testing.T) {
	clearEnv(t, buildkit.BuildCompressionEnvVar, buildkit.AlphaBuildCompressionEnvVar)

	exportPlatformVariablesToEnv([]env.Var{
		{Name: buildkit.AlphaBuildCompressionEnvVar, Value: "zstd"},
		{Name: buildkit.BuildCompressionEnvVar, Value: "zstd"},
	})

	require.Equal(t, "zstd", os.Getenv(buildkit.BuildCompressionEnvVar))
	require.Equal(t, "zstd", os.Getenv(buildkit.AlphaBuildCompressionEnvVar))
}

func Test_exportPlatformVariablesToEnv_OldPlatformSendsOnlyAlpha(t *testing.T) {
	clearEnv(t, buildkit.BuildCompressionEnvVar, buildkit.AlphaBuildCompressionEnvVar)

	exportPlatformVariablesToEnv([]env.Var{
		{Name: buildkit.AlphaBuildCompressionEnvVar, Value: "zstd"},
	})

	require.Equal(t, "zstd", os.Getenv(buildkit.AlphaBuildCompressionEnvVar))
	_, exists := os.LookupEnv(buildkit.BuildCompressionEnvVar)
	require.False(t, exists)
}

func Test_exportPlatformVariablesToEnv_UnrelatedVariableSetWhenAbsent(t *testing.T) {
	clearEnv(t, "OKTETO_TEST_UNRELATED_VAR")

	exportPlatformVariablesToEnv([]env.Var{
		{Name: "OKTETO_TEST_UNRELATED_VAR", Value: "platform"},
	})

	require.Equal(t, "platform", os.Getenv("OKTETO_TEST_UNRELATED_VAR"))
}

func Test_exportPlatformVariablesToEnv_UnrelatedVariableNotOverwritten(t *testing.T) {
	t.Setenv("OKTETO_TEST_UNRELATED_VAR", "local")

	exportPlatformVariablesToEnv([]env.Var{
		{Name: "OKTETO_TEST_UNRELATED_VAR", Value: "platform"},
	})

	require.Equal(t, "local", os.Getenv("OKTETO_TEST_UNRELATED_VAR"))
}

// When the platform sends the same variable more than once, the first value wins:
// a name exported earlier in the same call is not considered a local override,
// but it is not overwritten by later entries either
func Test_exportPlatformVariablesToEnv_DuplicateNameFirstWins(t *testing.T) {
	clearEnv(t, "OKTETO_TEST_DUPLICATED_VAR")

	exportPlatformVariablesToEnv([]env.Var{
		{Name: "OKTETO_TEST_DUPLICATED_VAR", Value: "first"},
		{Name: "OKTETO_TEST_DUPLICATED_VAR", Value: "second"},
	})

	require.Equal(t, "first", os.Getenv("OKTETO_TEST_DUPLICATED_VAR"))
}
