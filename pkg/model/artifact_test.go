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
