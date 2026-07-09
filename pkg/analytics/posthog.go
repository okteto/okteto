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

package analytics

import (
	"context"
	"maps"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	posthog "github.com/posthog/posthog-go"
)

const (
	// skipcq GSC-G101
	posthogToken = "phc_hABE2tLx6OC3RdADvVeFYfwtQQhYSE5swoqkQQscU6K"

	// posthogEndpoint is the Okteto-owned reverse proxy for PostHog.
	posthogEndpoint = "https://ph.okteto.com"

	posthogImageBuildEvent              = "image_build"
	posthogDeployPipelineTriggeredEvent = "deploy_pipeline_triggered"
	posthogDeployPreviewTriggeredEvent  = "deploy_preview_triggered"
	posthogUpEvent                      = "up"
	posthogUpStartedEvent               = "up_started"
	posthogWakeTriggeredEvent           = "wake_triggered"
)

// posthogEnqueuer is a narrow interface over posthog.Client that only exposes
// the methods we use. This keeps the mock simple in tests.
type posthogEnqueuer interface {
	Enqueue(posthog.Message) error
	Close() error
}

// enricherFn enriches a set of PostHog properties before the event is sent.
// Enrichers run inside the background goroutine, so long-running I/O (e.g. K8s
// API calls) does not block the caller.
type enricherFn func(ctx context.Context, props posthog.Properties)

// NamespaceUIDResolver fetches the UID of a Kubernetes namespace.
// Implementations are expected to cache results to avoid repeated API calls.
type NamespaceUIDResolver interface {
	GetNamespaceUID(ctx context.Context, namespace string) (string, error)
}

// posthogBackend sends analytics events to PostHog.
// posthog.New() initializes an HTTP client and background flush goroutine at
// construction time, consistent with how mixpanelClient is initialized via init().
type posthogBackend struct {
	client     posthogEnqueuer
	nsResolver NamespaceUIDResolver
	wg         sync.WaitGroup
}

// newPostHogBackend creates the PostHog backend.
// Returns a no-op backend (nil client) when the token is empty — safe for tests.
func newPostHogBackend() *posthogBackend {
	b := &posthogBackend{
		nsResolver: namespaces.NewUIDResolver(okteto.NewK8sClientProvider()),
	}
	if posthogToken == "" {
		return b
	}
	client, err := posthog.NewWithConfig(posthogToken, posthog.Config{
		Endpoint: posthogEndpoint,
	})
	if err != nil {
		oktetoLog.Infof("failed to create posthog client: %s", err)
		return b
	}
	b.client = client
	return b
}

// commonPostHogProperties returns properties sent on every PostHog event from the CLI.
// Must only be called after analyticsEnabled() returns true — GetContext() Fatalf's on
// an uninitialized context.
//
// user_id is included as an explicit property (per product spec) in addition to being
// set as DistinctId on posthog.Capture. agent_type is omitted when is_agent is false.
func commonPostHogProperties() posthog.Properties {
	ctx := okteto.GetContext()
	agent := getAgent()
	props := posthog.Properties{
		// Common (all PostHog sources)
		"customer_name": ctx.CompanyName,
		// customer_id is derived server-side in PostHog, not sent from the CLI.
		"cluster_id":      ctx.ClusterID,
		"cluster_url":     okteto.NormalizeClusterURL(ctx.Name),
		"cluster_version": ctx.ClusterVersion,
		"user_id":         ctx.UserID,

		// CLI common
		"cli_version":        config.VersionString,
		"os":                 runtime.GOOS,
		"arch":               runtime.GOARCH,
		"machine_id":         get().MachineID,
		"measurement_source": "cli",
		"trigger_source":     config.GetDeployOrigin(),
		"is_agent":           agent != "",
		"is_ci":              isCI(),
		"is_automation":      isCI() || agent != "",
	}
	if agent != "" {
		props["agent_type"] = agent
	}
	return props
}

// isCI reports whether the CLI is running inside a CI environment.
func isCI() bool {
	return env.LoadBoolean(constants.CIEnvVar)
}

func getAgent() string {
	if env.LoadBoolean("CODEX_CI") {
		return "codex"
	}
	if env.LoadBoolean("GEMINI_CLI") {
		return "gemini"
	}
	if env.LoadBoolean("CLAUDECODE") {
		return "claude_code"
	}
	if os.Getenv("CURSOR_SANDBOX") != "" {
		return "cursor"
	}
	return ""
}

// enqueue runs any enrichers then sends the event to PostHog in a background
// goroutine. The caller is never blocked by enricher I/O.
func (b *posthogBackend) enqueue(ctx context.Context, userID, event string, props posthog.Properties, enrichers ...enricherFn) {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for _, enrich := range enrichers {
			enrich(ctx, props)
		}
		if err := b.client.Enqueue(posthog.Capture{
			DistinctId: userID,
			Event:      event,
			Properties: props,
		}); err != nil {
			oktetoLog.Infof("failed to send posthog analytics: %s", err)
		}
	}()
}

// withNamespace returns an enricherFn that resolves the namespace UID and sets
// it as the "namespace" property. Sets an empty string when namespace is empty,
// the resolver is unavailable, or the lookup fails, so downstream can distinguish
// "no namespace" from "namespace not resolved".
func (b *posthogBackend) withNamespace(namespace string) enricherFn {
	return func(ctx context.Context, props posthog.Properties) {
		if namespace == "" || b.nsResolver == nil {
			props["namespace"] = ""
			return
		}
		fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if uid, err := b.nsResolver.GetNamespaceUID(fetchCtx, namespace); err != nil {
			oktetoLog.Infof("analytics: failed to get namespace UID: %s", err)
			props["namespace"] = ""
		} else {
			props["namespace"] = uid
		}
	}
}

// withPreview returns an enricherFn that resolves the preview namespace UID and sets
// it as the "preview" property. Sets an empty string when preview is empty,
// the resolver is unavailable, or the lookup fails.
func (b *posthogBackend) withPreview(previewName string) enricherFn {
	return func(ctx context.Context, props posthog.Properties) {
		if previewName == "" || b.nsResolver == nil {
			props["preview"] = ""
			return
		}
		fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if uid, err := b.nsResolver.GetNamespaceUID(fetchCtx, previewName); err != nil {
			oktetoLog.Infof("analytics: failed to get preview namespace UID: %s", err)
			props["preview"] = ""
		} else {
			props["preview"] = uid
		}
	}
}

// TrackImageBuild sends an image_build event to PostHog.
func (b *posthogBackend) TrackImageBuild(ctx context.Context, m *ImageBuildMetadata) {
	if b.client == nil {
		return
	}
	if !analyticsEnabled() {
		return
	}

	userID := okteto.GetContext().UserID
	props := commonPostHogProperties()
	maps.Copy(props, m.toPostHogProps())
	b.enqueue(ctx, userID, posthogImageBuildEvent, props, b.withNamespace(m.Namespace))
}

// TrackDeployPipelineTriggered sends a deploy_pipeline_triggered event to PostHog.
func (b *posthogBackend) TrackDeployPipelineTriggered(ctx context.Context, m DeployPipelineTriggeredMetadata) {
	if b.client == nil || !analyticsEnabled() {
		return
	}

	userID := okteto.GetContext().UserID
	props := commonPostHogProperties()
	props["workflow_id"] = m.WorkflowID
	props["is_within_preview"] = m.IsWithinPreview
	props["is_redeploy"] = m.IsRedeploy
	props["deploy_type"] = m.DeployType
	if m.ParentWorkflowID != "" {
		props["parent_workflow_id"] = m.ParentWorkflowID
	}
	if m.RepoURL != "" {
		props["repo_url"] = hashString(normalizeRepoURL(m.RepoURL))
	}
	b.enqueue(ctx, userID, posthogDeployPipelineTriggeredEvent, props, b.withNamespace(m.Namespace))
}

// TrackDeployPreviewTriggered sends a deploy_preview_triggered event to PostHog.
func (b *posthogBackend) TrackDeployPreviewTriggered(ctx context.Context, m DeployPreviewTriggeredMetadata) {
	if b.client == nil || !analyticsEnabled() {
		return
	}

	userID := okteto.GetContext().UserID
	props := commonPostHogProperties()
	props["workflow_id"] = m.WorkflowID
	props["is_redeploy"] = m.IsRedeploy
	if m.ParentWorkflowID != "" {
		props["parent_workflow_id"] = m.ParentWorkflowID
	}
	if m.RepoURL != "" {
		props["repo_url"] = hashString(normalizeRepoURL(m.RepoURL))
	}
	b.enqueue(ctx, userID, posthogDeployPreviewTriggeredEvent, props, b.withPreview(m.Preview))
}

// TrackWakeTriggered sends a wake_triggered event to PostHog.
func (b *posthogBackend) TrackWakeTriggered(ctx context.Context, m WakeTriggeredMetadata) {
	if b.client == nil || !analyticsEnabled() {
		return
	}

	userID := okteto.GetContext().UserID
	props := commonPostHogProperties()
	props["is_preview"] = m.IsPreview
	b.enqueue(ctx, userID, posthogWakeTriggeredEvent, props, b.withNamespace(m.Namespace))
}

// IsWithinPreview reports whether the current CLI context is inside a preview
// environment. It first checks the platform env var (set when running inside a
// preview); if not set, it calls checkPreview with the active namespace so that
// `okteto ns use <preview-name>` is also detected. Pass nil to skip the API check.
func IsWithinPreview(ctx context.Context, checkPreview func(context.Context, string) error) bool {
	if os.Getenv(constants.OktetoIsPreviewEnvVar) == "true" {
		return true
	}
	if checkPreview == nil {
		return false
	}
	ns := okteto.GetContext().Namespace
	if ns == "" {
		return false
	}
	return checkPreview(ctx, ns) == nil
}

// TrackUp sends an up event to PostHog.
func (b *posthogBackend) TrackUp(m *UpMetricsMetadata) {
	if b.client == nil {
		return
	}
	if !analyticsEnabled() {
		return
	}
	userID := okteto.GetContext().UserID
	props := commonPostHogProperties()
	maps.Copy(props, m.toPostHogProps())
	b.enqueue(context.Background(), userID, posthogUpEvent, props, b.withNamespace(m.namespace))
}

// TrackDeployStarted sends a deploy_started event to PostHog at the beginning of the deploy command.
func (b *posthogBackend) TrackDeployStarted(m DeployStartedMetadata) {
	if b.client == nil {
		return
	}
	if !analyticsEnabled() {
		return
	}
	userID := okteto.GetContext().UserID
	props := commonPostHogProperties()
	maps.Copy(props, m.toPostHogProps())
	b.enqueue(context.Background(), userID, posthogDeployStartedEvent, props, b.withNamespace(m.Namespace))
}

// TrackDeploy sends a deploy_completed event to PostHog.
func (b *posthogBackend) TrackDeploy(m DeployMetadata) {
	if b.client == nil {
		return
	}
	if !analyticsEnabled() {
		return
	}
	userID := okteto.GetContext().UserID
	props := commonPostHogProperties()
	maps.Copy(props, m.toPostHogProps())
	b.enqueue(context.Background(), userID, posthogDeployCompletedEvent, props, b.withNamespace(m.Namespace))
}

// TrackUpStarted sends an up_started event to PostHog at the beginning of the up command.
func (b *posthogBackend) TrackUpStarted(service, namespace, repoURL, workflowID string) {
	if b.client == nil {
		return
	}
	if !analyticsEnabled() {
		return
	}
	userID := okteto.GetContext().UserID
	props := commonPostHogProperties()
	hashedRepoURL := ""
	if repoURL != "" {
		hashedRepoURL = hashString(normalizeRepoURL(repoURL))
	}
	props["service"] = service
	props["repo_url"] = hashedRepoURL
	props["workflow_id"] = workflowID
	b.enqueue(context.Background(), userID, posthogUpStartedEvent, props, b.withNamespace(namespace))
}

// Close waits for any in-flight goroutines to finish enqueuing, then flushes
// and shuts down the PostHog client.
func (b *posthogBackend) Close() {
	if b.client == nil {
		return
	}
	b.wg.Wait()
	if err := b.client.Close(); err != nil {
		oktetoLog.Infof("failed to close posthog client: %s", err)
	}
}
