// Copyright 2023-2025 The Okteto Authors
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

//go:build !windows
// +build !windows

package plugin

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// With the gate off, MaybeExec must touch nothing: no okteto-home resolution
// (which would create the folder), no PATH lookup, no exec. Pins invariant:
// disabled feature has zero side effects.
func TestMaybeExecGateOffHasNoSideEffects(t *testing.T) {
	tests := []struct {
		name string
		gate string
	}{
		{name: "unset", gate: ""},
		{name: "explicit false", gate: "false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("OKTETO_HOME", home)
			t.Setenv(OktetoAlphaPluginEnabledEnvVar, tt.gate)

			MaybeExec(newTestRoot())

			entries, err := os.ReadDir(home)
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}
