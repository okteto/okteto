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
	"context"
	"errors"
	"time"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
)

const (
	deployEvent = "Deploy"
)

// DeployMetadata contains the metadata of a deploy event
type DeployMetadata struct {
	Err                    error
	PipelineType           model.Archetype
	DeployType             string
	Namespace              string
	RepoURL                string
	ManifestSyntax         string
	WorkflowID             string
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

// errorCategory returns the category of the deploy error, or "unknown" for
// errors it doesn't classify. The raw error is never sent.
func (d *DeployMetadata) errorCategory() string {
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
		return "unknown"
	}
}

// toProps returns the properties of the Deploy event: the Mixpanel ones plus
// the ones that PostHog's deploy_completed had and Deploy didn't. The ported
// optional ones are omitted when empty.
func (d *DeployMetadata) toProps() map[string]any {
	props := map[string]any{
		"pipelineType":           d.manifestArchetype(),
		"isOktetoRepository":     d.IsOktetoRepo,
		"duration":               d.Duration.Seconds(),
		"deployType":             d.DeployType,
		"isPreview":              d.IsPreview,
		"hasDependenciesSection": d.HasDependenciesSection,
		"hasBuildSection":        d.HasBuildSection,
		"isRemote":               d.IsRemote,

		"result":        d.Success,
		"isRedeploy":    d.IsRedeploy,
		"namespaceType": d.namespaceType(),
	}
	if d.Err != nil {
		// Never the raw error: it can hold command lines, URLs and credentials.
		props["error"] = d.errorCategory()
	}
	if d.RepoURL != "" {
		props["repoURL"] = hashString(normalizeRepoURL(d.RepoURL))
	}
	if d.ManifestSyntax != "" {
		props["manifestSyntax"] = d.ManifestSyntax
	}
	if d.WorkflowID != "" {
		props["workflowID"] = d.WorkflowID
	}
	if d.ParentExecutionID != "" {
		props["parentExecutionID"] = d.ParentExecutionID
	}
	if d.WaitForDependencies {
		props["waitForDependencies"] = true
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
	WorkflowID        string
	ParentExecutionID string
	IsPreview         bool
	IsRedeploy        bool
}

// toProductEventProps returns the properties of the deploy_started product event.
func (d *DeployStartedMetadata) toProductEventProps() map[string]any {
	props := map[string]any{
		"isPreview":  d.IsPreview,
		"isRedeploy": d.IsRedeploy,
	}
	if d.RepoURL != "" {
		props["repoURL"] = hashString(normalizeRepoURL(d.RepoURL))
	}
	if d.WorkflowID != "" {
		props["workflowID"] = d.WorkflowID
	}
	if d.ParentExecutionID != "" {
		props["parentExecutionID"] = d.ParentExecutionID
	}
	return props
}

// TrackDeployStarted fires the deploy_started event at the beginning of the deploy command.
func (a *Tracker) TrackDeployStarted(metadata DeployStartedMetadata) {
	a.events.track(context.Background(), productDeployStartedEvent, metadata.toProductEventProps(), a.events.withNamespace(metadata.Namespace))
}

// TrackDeploy sends the Deploy event when the user deploys from command okteto deploy
func (a *Tracker) TrackDeploy(metadata DeployMetadata) {
	a.events.trackWithProductProperties(context.Background(), deployEvent, metadata.Success, metadata.toProps(), a.events.withNamespace(metadata.Namespace))
}
