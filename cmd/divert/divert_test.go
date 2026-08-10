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

package divert

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/log/io"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func newDivertCommand() *cobra.Command {
	return Divert(context.Background(), io.NewIOController())
}

func subcommand(t *testing.T, name string) *cobra.Command {
	t.Helper()

	for _, c := range newDivertCommand().Commands() {
		if c.Name() == name {
			return c
		}
	}

	t.Fatalf("subcommand %q not found", name)

	return nil
}

func requireRequiredFlag(t *testing.T, cmd *cobra.Command, name string) {
	t.Helper()

	flag := cmd.Flags().Lookup(name)
	require.NotNil(t, flag, "flag %q not found", name)
	require.Equal(t, []string{"true"}, flag.Annotations[cobra.BashCompOneRequiredFlag])
}

func TestDivert_HasUpAndDown(t *testing.T) {
	cmd := newDivertCommand()

	require.Equal(t, "divert", cmd.Use)
	require.False(t, cmd.Hidden)
	require.Len(t, cmd.Commands(), 2)
}

func TestUp_Flags(t *testing.T) {
	cmd := subcommand(t, "up")

	requireRequiredFlag(t, cmd, "service")
	requireRequiredFlag(t, cmd, "from")
	require.NotNil(t, cmd.Flags().Lookup("to"))
	require.NotNil(t, cmd.Flags().Lookup("key"))
	require.NotNil(t, cmd.Flags().Lookup("timeout"))
}

// --force was deliberately dropped from Phase 0: teardown already reads nothing but the
// annotations, so the flag had no behaviour left that was not harmful.
func TestDown_Flags(t *testing.T) {
	cmd := subcommand(t, "down")

	requireRequiredFlag(t, cmd, "service")
	requireRequiredFlag(t, cmd, "from")
	require.NotNil(t, cmd.Flags().Lookup("key"))
	require.NotNil(t, cmd.Flags().Lookup("all"))
	require.Nil(t, cmd.Flags().Lookup("force"))
}

func TestUp_HasANoRestartFlag(t *testing.T) {
	require.NotNil(t, subcommand(t, "up").Flags().Lookup("no-restart"))
}

// --all means "everyone's routes", so it must not carry one developer's key with it.
func TestDownRoutingKey_IsEmptyWhenTearingEverythingDown(t *testing.T) {
	require.Empty(t, downRoutingKey(&downFlags{all: true, key: "alice"}))
}

func TestDownRoutingKey_PrefersAnExplicitKey(t *testing.T) {
	require.Equal(t, "alice", downRoutingKey(&downFlags{key: "alice"}))
}

func TestUp_RejectsPositionalArguments(t *testing.T) {
	require.Error(t, subcommand(t, "up").Args(subcommand(t, "up"), []string{"api"}))
}

func TestDown_RejectsPositionalArguments(t *testing.T) {
	require.Error(t, subcommand(t, "down").Args(subcommand(t, "down"), []string{"api"}))
}

func TestRoutingKeyOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		target   string
		expected string
	}{
		{
			name:     "explicit key wins",
			key:      "alice",
			target:   "alice-dev",
			expected: "alice",
		},
		{
			name:     "defaults to the target namespace, not the current one",
			key:      "",
			target:   "alice-dev",
			expected: "alice-dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, routingKeyOrDefault(tt.key, tt.target))
		})
	}
}

func TestTargetNamespace_PrefersTheFlag(t *testing.T) {
	require.Equal(t, "alice-dev", targetNamespace("alice-dev"))
}
