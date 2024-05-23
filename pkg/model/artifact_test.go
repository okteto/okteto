// Copyright 2024 The Okteto Authors
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

package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArtifact(t *testing.T) {
	data := []byte(`
dev: {}
test:
  unit:
    commands: []
    artifacts:
      - report
`)
	manifest, err := Read(data)
	require.NoError(t, err)

	require.Equal(t, "report", manifest.Test["unit"].Artifacts[0].Destination)
	require.Equal(t, "report", manifest.Test["unit"].Artifacts[0].Path)
}

func TestArtifactExtended(t *testing.T) {
	data := []byte(`
dev: {}
test:
  unit:
    commands: []
    artifacts:
      - path: report
        destination: out
`)
	manifest, err := Read(data)
	require.NoError(t, err)

	require.Equal(t, "out", manifest.Test["unit"].Artifacts[0].Destination)
	require.Equal(t, "report", manifest.Test["unit"].Artifacts[0].Path)
}

func TestArtifactExtendedNoDest(t *testing.T) {
	data := []byte(`
dev: {}
test:
  unit:
    commands: []
    artifacts:
      - path: report
`)
	manifest, err := Read(data)
	require.NoError(t, err)

	require.Equal(t, "report", manifest.Test["unit"].Artifacts[0].Destination)
	require.Equal(t, "report", manifest.Test["unit"].Artifacts[0].Path)
}
