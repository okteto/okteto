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

package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeStateFile writes a state file into a temp dir that mimics the
// <OKTETO_FOLDER>/<namespace>/<name>/okteto.state structure.
func writeStateFile(t *testing.T, home, ns, name string, state config.UpState) {
	t.Helper()
	dir := filepath.Join(home, ns, name)
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "okteto.state"), []byte(state), 0600))
}

// setOktetoFolder points OKTETO_FOLDER at dir and restores the original value on cleanup.
func setOktetoFolder(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("OKTETO_FOLDER", dir)
}

func TestPrintState_Running(t *testing.T) {
	tests := []struct {
		state   config.UpState
		wantNil bool
		wantMsg string
	}{
		{config.Activating, true, "activating dev container"},
		{config.Starting, true, "scheduling pod"},
		{config.Attaching, true, "attaching persistent volume"},
		{config.Pulling, true, "pulling image"},
		{config.StartingSync, true, "initialising file sync"},
		{config.Synchronizing, true, "running and syncing files"},
		{config.Ready, true, "is running"},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			err := printState("nginx", tt.state)
			assert.NoError(t, err)
		})
	}
}

func TestPrintState_Failed(t *testing.T) {
	err := printState("nginx", config.Failed)
	require.Error(t, err)

	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	assert.Contains(t, userErr.E.Error(), "nginx")
}

func TestPrintState_Unknown(t *testing.T) {
	err := printState("nginx", config.UpState("weird-value"))
	assert.NoError(t, err)
}

func TestInfo_StateFile_Absent(t *testing.T) {
	tmp := t.TempDir()
	setOktetoFolder(t, tmp)

	statePath := filepath.Join(tmp, "ns", "nginx", "okteto.state")
	_, err := os.Stat(statePath)
	require.True(t, os.IsNotExist(err), "state file must not exist before the test")
}

func TestInfo_StateFile_AllStates(t *testing.T) {
	tests := []struct {
		state   config.UpState
		wantErr bool
	}{
		{config.Activating, false},
		{config.Starting, false},
		{config.Attaching, false},
		{config.Pulling, false},
		{config.StartingSync, false},
		{config.Synchronizing, false},
		{config.Ready, false},
		{config.Failed, true},
		{config.UpState("mystery"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			tmp := t.TempDir()
			setOktetoFolder(t, tmp)

			writeStateFile(t, tmp, "ns", "nginx", tt.state)

			// Verify the state file is readable and GetState returns the expected value.
			got, err := config.GetState("nginx", "ns")
			require.NoError(t, err)
			assert.Equal(t, tt.state, got)

			// Verify printState returns the correct error/nil.
			printErr := printState("nginx", got)
			if tt.wantErr {
				require.Error(t, printErr)
			} else {
				assert.NoError(t, printErr)
			}
		})
	}
}
