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

	"github.com/dukex/mixpanel"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

// Product events. They used to be sent to PostHog and now go to Mixpanel with
// the same names and properties, so the reports built on them keep working.
// They don't go through track(): they carry their own common
// properties (commonProductEventProperties) instead of the ones track() adds.
const (
	productImageBuildEvent              = "image_build"
	productDeployStartedEvent           = "deploy_started"
	productDeployCompletedEvent         = "deploy_completed"
	productDeployPipelineTriggeredEvent = "deploy_pipeline_triggered"
	productDeployPreviewTriggeredEvent  = "deploy_preview_triggered"
	productUpStartedEvent               = "up_started"
	productUpEvent                      = "up"
	productWakeTriggeredEvent           = "wake_triggered"

	// productEventsCloseTimeout bounds how long Close waits for in-flight events,
	// so a slow network can't hold the CLI on exit.
	productEventsCloseTimeout = 5 * time.Second

	// namespaceUIDTimeout bounds the namespace UID lookup of an enricher.
	namespaceUIDTimeout = 5 * time.Second
)

// eventTracker is the part of mixpanel.Mixpanel the product events use.
type eventTracker interface {
	Track(distinctID, eventName string, e *mixpanel.Event) error
}

// enricherFn adds properties to an event before it is sent. Enrichers run in
// the event's goroutine, so their I/O (e.g. K8s API calls) doesn't block the caller.
type enricherFn func(ctx context.Context, props map[string]any)

// NamespaceUIDResolver fetches the UID of a Kubernetes namespace.
// Implementations are expected to cache results to avoid repeated API calls.
type NamespaceUIDResolver interface {
	GetNamespaceUID(ctx context.Context, namespace string) (string, error)
}

// productEventClient sends product events to Mixpanel without blocking the caller:
// each event is enriched and sent in its own goroutine, and Close waits for the
// pending ones up to productEventsCloseTimeout.
// A nil *productEventClient is valid and drops every event.
type productEventClient struct {
	client       eventTracker
	nsResolver   NamespaceUIDResolver
	now          func() time.Time
	closeTimeout time.Duration

	// mu guards closed and the wg.Add calls, so no event is added after Close starts waiting.
	mu     sync.Mutex
	wg     sync.WaitGroup
	closed bool
}

func newProductEventClient() *productEventClient {
	return &productEventClient{
		client:       mixpanelClient,
		nsResolver:   namespaces.NewUIDResolver(okteto.NewK8sClientProvider()),
		now:          time.Now,
		closeTimeout: productEventsCloseTimeout,
	}
}

// track sends event with the common product properties plus eventProps.
// Events are dropped when analytics is disabled, when there is no user ID
// (the distinct ID, as in PostHog) and after Close.
func (c *productEventClient) track(ctx context.Context, event string, eventProps map[string]any, enrichers ...enricherFn) {
	if c == nil || c.client == nil || !analyticsEnabled() {
		return
	}
	distinctID := okteto.GetContext().UserID
	if distinctID == "" {
		oktetoLog.Infof("analytics: dropping %s event: no user id", event)
		return
	}
	props := commonProductEventProperties()
	maps.Copy(props, eventProps)
	// The event time is taken here, not when the goroutine sends it, so a slow
	// enricher or request doesn't shift it (Mixpanel stores it in seconds).
	ts := c.now()

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		oktetoLog.Infof("analytics: dropping %s event: tracked after close", event)
		return
	}
	c.wg.Add(1)
	c.mu.Unlock()

	go func() {
		defer c.wg.Done()
		for _, enrich := range enrichers {
			enrich(ctx, props)
		}
		if err := c.client.Track(distinctID, event, &mixpanel.Event{Timestamp: &ts, Properties: props}); err != nil {
			oktetoLog.Infof("failed to send %s analytics event: %s", event, err)
		}
	}()
}

// Close stops accepting events and waits for the pending ones, up to closeTimeout.
func (c *productEventClient) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(c.closeTimeout):
		oktetoLog.Infof("analytics: timed out after %s waiting for pending events", c.closeTimeout)
	}
}

// withNamespaceUID returns an enricherFn that resolves nsName's UID into propKey,
// or "" when nsName is empty, the resolver is nil, or the lookup fails.
// The namespace name is never sent.
func (c *productEventClient) withNamespaceUID(propKey, nsName string) enricherFn {
	return func(ctx context.Context, props map[string]any) {
		if nsName == "" || c.nsResolver == nil {
			props[propKey] = ""
			return
		}
		fetchCtx, cancel := context.WithTimeout(ctx, namespaceUIDTimeout)
		defer cancel()
		uid, err := c.nsResolver.GetNamespaceUID(fetchCtx, nsName)
		if err != nil {
			oktetoLog.Infof("analytics: failed to get %s UID: %s", propKey, err)
			props[propKey] = ""
			return
		}
		props[propKey] = uid
	}
}

// withNamespace resolves the namespace UID into the "namespace" property.
func (c *productEventClient) withNamespace(namespace string) enricherFn {
	return c.withNamespaceUID("namespace", namespace)
}

// withPreview resolves the preview namespace UID into the "preview" property.
func (c *productEventClient) withPreview(previewName string) enricherFn {
	return c.withNamespaceUID("preview", previewName)
}

// commonProductEventProperties returns the properties sent on every product event,
// with the names PostHog used. Some duplicate a property track() sends on the
// other Mixpanel events (customer_name/context, cli_version/version, os/$os,
// trigger_source/origin); they are kept so the product events stay as they were.
// Must only be called after analyticsEnabled() returns true: GetContext() Fatalf's
// on an uninitialized context.
//
// agent_type is omitted when is_agent is false.
func commonProductEventProperties() map[string]any {
	ctx := okteto.GetContext()
	agent := getAgent()
	ci := isCI()
	props := map[string]any{
		"customer_name":   ctx.CompanyName,
		"cluster_id":      ctx.ClusterID,
		"cluster_url":     okteto.NormalizeClusterURL(ctx.Name),
		"cluster_version": ctx.ClusterVersion,
		"user_id":         ctx.UserID,

		"cli_version":        config.VersionString,
		"os":                 runtime.GOOS,
		"arch":               runtime.GOARCH,
		"machine_id":         get().MachineID,
		"measurement_source": "cli",
		"trigger_source":     config.GetDeployOrigin(),
		"is_agent":           agent != "",
		"is_ci":              ci,
		"is_automation":      ci || agent != "",
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

// getAgent returns the AI coding agent running the CLI, or "" when there is none.
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
