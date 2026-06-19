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
	"time"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
)

const (
	deployEvent                 = "Deploy"
	posthogDeployStartedEvent   = "deploy_started"
	posthogDeployCompletedEvent = "deploy_completed"
)

// DeployMetadata contains the metadata of a deploy event
type DeployMetadata struct {
	Err                    error
	PipelineType           model.Archetype
	DeployType             string
	Namespace              string
	RepoURL                string
	ManifestSyntax         string
	ParentExecutionID      string
	Duration               time.Duration
	Success                bool
	IsOktetoRepo           bool
	IsPreview              bool
	IsRedeploy             bool
	HasDependenciesSection bool
	HasBuildSection        bool
	IsRemote               bool
	WaitForDependencies    bool
}

func (d *DeployMetadata) errorReason() string {
	switch {
	case errors.Is(d.Err, oktetoErrors.ErrManifestFoundButNoDeployAndDependenciesCommands):
		return "no_deploy_commands"
	case errors.Is(d.Err, oktetoErrors.ErrTimeout):
		return "timeout"
	case errors.Is(d.Err, oktetoErrors.ErrCommandFailed):
		return "command_failed"
	case errors.Is(d.Err, oktetoErrors.ErrInternalServerError):
		return "internal_server_error"
	case errors.As(d.Err, &oktetoErrors.UserError{}):
		return "user_error"
	default:
		return ""
	}
}

func (d *DeployMetadata) toPostHogProps() map[string]any {
	props := map[string]any{
		"result":                   d.Success,
		"manifest_archetype":       d.manifestArchetype(),
		"is_within_preview":        d.IsPreview,
		"is_redeploy":              d.IsRedeploy,
		"has_dependencies_section": d.HasDependenciesSection,
		"has_build_section":        d.HasBuildSection,
		"is_remote":                d.IsRemote,
		"namespace_type":           d.namespaceType(),
	}
	if d.RepoURL != "" {
		props["repo_url"] = hashString(normalizeRepoURL(d.RepoURL))
	}
	if d.ManifestSyntax != "" {
		props["manifest_syntax"] = d.ManifestSyntax
	}
	if d.ParentExecutionID != "" {
		props["parent_execution_id"] = d.ParentExecutionID
	}
	if d.WaitForDependencies {
		props["wait_for_dependencies"] = true
	}
	if secs := d.Duration.Seconds(); secs > 0 {
		props["duration_seconds"] = secs
	}
	if !d.Success {
		if reason := d.errorReason(); reason != "" {
			props["error_reason"] = reason
		}
	}
	return props
}

func (d *DeployMetadata) manifestArchetype() string {
	if d.PipelineType == "" {
		return "pipeline"
	}
	return string(d.PipelineType)
}

func (d *DeployMetadata) namespaceType() string {
	if d.IsPreview {
		return "preview"
	}
	return "regular"
}

// DeployStartedMetadata contains the metadata of a deploy_started event, fired at
// the beginning of the deploy command before the deploy runs.
type DeployStartedMetadata struct {
	Namespace         string
	RepoURL           string
	ParentExecutionID string
	IsPreview         bool
	IsRedeploy        bool
}

func (d *DeployStartedMetadata) toPostHogProps() map[string]any {
	props := map[string]any{
		"is_within_preview": d.IsPreview,
		"is_redeploy":       d.IsRedeploy,
	}
	if d.RepoURL != "" {
		props["repo_url"] = hashString(normalizeRepoURL(d.RepoURL))
	}
	if d.ParentExecutionID != "" {
		props["parent_execution_id"] = d.ParentExecutionID
	}
	return props
}

// TrackDeployStarted fires the deploy_started event at the beginning of the deploy command.
func (a *Tracker) TrackDeployStarted(metadata DeployStartedMetadata) {
	for _, b := range a.backends {
		b.TrackDeployStarted(metadata)
	}
}

// TrackDeploy sends a tracking event to mixpanel when the user deploys from command okteto deploy
func (a *Tracker) TrackDeploy(metadata DeployMetadata) {
	if metadata.PipelineType == "" {
		metadata.PipelineType = "pipeline"
	}
	props := map[string]any{
		"pipelineType":           metadata.PipelineType,
		"isOktetoRepository":     metadata.IsOktetoRepo,
		"duration":               metadata.Duration.Seconds(),
		"deployType":             metadata.DeployType,
		"isPreview":              metadata.IsPreview,
		"hasDependenciesSection": metadata.HasDependenciesSection,
		"hasBuildSection":        metadata.HasBuildSection,
		"isRemote":               metadata.IsRemote,
	}
	if metadata.Err != nil {
		props["error"] = metadata.Err.Error()
	}
	a.trackFn(deployEvent, metadata.Success, props)
	for _, b := range a.backends {
		b.TrackDeploy(metadata)
	}
}
