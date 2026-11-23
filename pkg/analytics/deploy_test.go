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

package analytics

import (
	"errors"
	"fmt"
	"testing"
	"time"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployMetadata_ToProps(t *testing.T) {
	props := (&DeployMetadata{
		Success:                true,
		IsOktetoRepo:           true,
		Duration:               2 * time.Second,
		PipelineType:           model.StackType,
		DeployType:             "deploy",
		IsPreview:              true,
		IsRedeploy:             true,
		HasDependenciesSection: true,
		HasBuildSection:        true,
		IsRemote:               true,
		RepoURL:                "https://github.com/org/repo",
		ManifestSyntax:         "compose",
		WorkflowID:             "wf-1",
		ParentExecutionID:      "exec-1",
		WaitForDependencies:    true,
	}).toProps()

	require.Equal(t, map[string]any{
		"pipelineType":           "compose",
		"isOktetoRepository":     true,
		"duration":               2.0,
		"deployType":             "deploy",
		"isPreview":              true,
		"hasDependenciesSection": true,
		"hasBuildSection":        true,
		"isRemote":               true,
		"isRedeploy":             true,
		"namespaceType":          "preview",
		"repoURL":                hashedTestRepoURL,
		"manifestSyntax":         "compose",
		"workflowID":             "wf-1",
		"parentExecutionID":      "exec-1",
		"waitForDependencies":    true,
	}, props)
}

func TestDeployMetadata_ToProps_DefaultsAndOmittedFields(t *testing.T) {
	props := (&DeployMetadata{Success: true}).toProps()

	require.Equal(t, map[string]any{
		"pipelineType":           "pipeline",
		"isOktetoRepository":     false,
		"duration":               0.0,
		"deployType":             "",
		"isPreview":              false,
		"hasDependenciesSection": false,
		"hasBuildSection":        false,
		"isRemote":               false,
		"isRedeploy":             false,
		"namespaceType":          "regular",
	}, props)
}

func TestDeployMetadata_ToProps_ErrorIsCategorized(t *testing.T) {
	props := (&DeployMetadata{Err: oktetoErrors.UserError{E: errors.New("error executing command './deploy.sh --password=pass': exit status 1")}}).toProps()

	require.Equal(t, "user_error", props["error"])
}

func TestErrorCategory(t *testing.T) {
	tests := []struct {
		err      error
		name     string
		expected string
	}{
		{name: "no deploy commands", err: oktetoErrors.ErrManifestFoundButNoDeployAndDependenciesCommands, expected: "no_deploy_commands"},
		{name: "timeout", err: fmt.Errorf("deploy: %w", oktetoErrors.ErrTimeout), expected: "timeout"},
		{name: "command failed", err: oktetoErrors.ErrCommandFailed, expected: "command_failed"},
		{name: "internal server error", err: oktetoErrors.ErrInternalServerError, expected: "internal_server_error"},
		{name: "user error", err: oktetoErrors.UserError{E: errors.New("bad manifest")}, expected: "user_error"},
		{name: "unclassified error", err: assert.AnError, expected: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, errorCategory(tt.err))
		})
	}
}

func TestDeployStartedMetadata_ToProductEventProps(t *testing.T) {
	props := (&DeployStartedMetadata{
		RepoURL:           "https://github.com/org/repo",
		WorkflowID:        "wf-1",
		ParentExecutionID: "exec-1",
		IsPreview:         true,
		IsRedeploy:        true,
	}).toProductEventProps()

	require.Equal(t, map[string]any{
		"isPreview":         true,
		"isRedeploy":        true,
		"repoURL":           hashedTestRepoURL,
		"workflowID":        "wf-1",
		"parentExecutionID": "exec-1",
	}, props)
}

func TestDeployStartedMetadata_ToProductEventProps_OmitsEmptyFields(t *testing.T) {
	props := (&DeployStartedMetadata{}).toProductEventProps()

	require.Equal(t, map[string]any{"isPreview": false, "isRedeploy": false}, props)
}
