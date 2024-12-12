//go:build integration
// +build integration

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

package okteto

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/require"
)

// TestGenerateSchema checks that the CLI can generate the Okteto Manifest JSON schema file successfully
func TestGenerateSchema(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	outputFile := filepath.Join(dir, "schema.json")

	generateSchemaOptions := &commands.GenerateSchemaOptions{
		Workdir:    dir,
		OutputFile: outputFile,
		OktetoHome: dir,
	}

	err = commands.RunOktetoGenerateSchema(oktetoPath, generateSchemaOptions)
	require.NoError(t, err)

	_, err = os.Stat(outputFile)
	require.NoError(t, err)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	var jsonSchema map[string]interface{}
	err = json.Unmarshal(content, &jsonSchema)
	require.NoError(t, err)

	// ensure only expected root-level properties are included
	rootLevelProps := []string{"build", "dependencies", "deploy", "destroy", "dev", "external", "forward", "icon", "name", "test"}
	for _, prop := range rootLevelProps {
		require.Contains(t, jsonSchema["properties"], prop)
	}
	require.Len(t, jsonSchema["properties"], len(rootLevelProps))
}

// TestGenerateSchemaStdout checks that the CLI can output the Okteto Manifest JSON schema to stdout successfully
func TestGenerateSchemaStdout(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	generateSchemaOptions := &commands.GenerateSchemaOptions{
		Workdir:    dir,
		OutputFile: "-", // output to stdout
		OktetoHome: dir,
	}

	output, err := commands.GetOktetoGenerateSchemaCmdOutput(oktetoPath, generateSchemaOptions)
	require.NoError(t, err)

	var jsonSchema map[string]interface{}
	err = json.Unmarshal(output, &jsonSchema)
	require.NoError(t, err)

	// ensure only expected root-level properties are included
	rootLevelProps := []string{"build", "dependencies", "deploy", "destroy", "dev", "external", "forward", "icon", "name", "test"}
	for _, prop := range rootLevelProps {
		require.Contains(t, jsonSchema["properties"], prop)
	}
	require.Len(t, jsonSchema["properties"], len(rootLevelProps))
}

// TestGenerateSchemaInvalidPath checks that when specifying an invalid path for the output file, the CLI returns an error
func TestGenerateSchemaInvalidPath(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	invalidPath := filepath.Join(dir, "non-existent-dir", "schema.json")

	generateSchemaOptions := &commands.GenerateSchemaOptions{
		Workdir:    dir,
		OutputFile: invalidPath,
		OktetoHome: dir,
	}

	err = commands.RunOktetoGenerateSchema(oktetoPath, generateSchemaOptions)
	require.Error(t, err)
}
