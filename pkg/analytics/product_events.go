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

// Product events: the events that used to go to PostHog and have no Mixpanel
// counterpart. They keep their PostHog names, with the properties in camelCase.
// They don't go through track(): they carry their own common properties
// (commonProductEventProperties) instead of the ones track() adds.
// The PostHog events fired from the same call as a Mixpanel event (image_build,
// deploy_completed and up) are now properties of imageBuild, Deploy and Up
// (see trackWithProductProperties).
const (
	productDeployStartedEvent           = "deploy_started"
	productDeployPipelineTriggeredEvent = "deploy_pipeline_triggered"
	productDeployPreviewTriggeredEvent  = "deploy_preview_triggered"
	productUpStartedEvent               = "up_started"
	productWakeTriggeredEvent           = "wake_triggered"

	// productEventsCloseTimeout bounds how long Close waits for in-flight events,
	// so a slow network can't hold the CLI on exit.
	productEventsCloseTimeout = 5 * time.Second

	// namespaceUIDTimeout bounds the namespace UID lookup of an enricher. It is
	// well under productEventsCloseTimeout, so an event whose lookup fails (e.g. the
	// cluster API is unreachable) is still sent, without the UID, before Close gives up.
	namespaceUIDTimeout = 2 * time.Second

	// noGeolocationIP tells the Mixpanel client not to ask Mixpanel to geolocate
	// the event from the request IP. posthog-go disabled GeoIP by default, so the
	// product events were never geolocated.
	noGeolocationIP = "0"
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

// track sends a product event with the common product properties plus eventProps.
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
	c.send(ctx, distinctID, event, noGeolocationIP, &ts, props, enrichers)
}

// trackWithProductProperties sends a Mixpanel event that PostHog also tracked
// (imageBuild, Deploy, Up) in the background. The rest is what track() does for
// it: its common properties, its distinct ID (the user ID, or the machine ID
// when there is none), geolocation, and no client timestamp (Mixpanel uses the
// time it receives the event, so a machine with a wrong clock doesn't lose it).
// The common product properties and the enrichers only apply to Okteto contexts,
// as PostHog only received these events from them: other contexts send what
// they sent before, without their kubeconfig context name or a namespace lookup.
// Events are dropped when analytics is disabled and after Close.
func (c *productEventClient) trackWithProductProperties(ctx context.Context, event string, success bool, eventProps map[string]any, enrichers ...enricherFn) {
	if c == nil || c.client == nil || !analyticsEnabled() {
		return
	}
	props := map[string]any{}
	if okteto.IsOkteto() {
		props = commonProductEventProperties()
	} else {
		enrichers = nil
	}
	maps.Copy(props, eventProps)
	addTrackProperties(props, success)
	c.send(ctx, getTrackID(), event, "", nil, props, enrichers)
}

// send runs the enrichers, drops unsafe property values (dropUnsafeProperties)
// and sends the event in its own goroutine. An empty ip
// lets Mixpanel geolocate the event from the request IP, and a nil ts lets
// Mixpanel use the time it receives the event.
func (c *productEventClient) send(ctx context.Context, distinctID, event, ip string, ts *time.Time, props map[string]any, enrichers []enricherFn) {

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
		dropUnsafeProperties(event, props)
		if err := c.client.Track(distinctID, event, &mixpanel.Event{IP: ip, Timestamp: ts, Properties: props}); err != nil {
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

// commonProductEventProperties returns the properties PostHog sent on every
// product event, in camelCase. Some duplicate a property track() sends
// (customerName/context, cliVersion/version, os/$os, triggerSource/origin);
// they are kept for now. machine_id keeps the name track() uses.
// Must only be called after analyticsEnabled() returns true: GetContext() Fatalf's
// on an uninitialized context.
//
// agentType is omitted when isAgent is false.
func commonProductEventProperties() map[string]any {
	ctx := okteto.GetContext()
	agent := getAgent()
	ci := isCI()
	props := map[string]any{
		"customerName":   ctx.CompanyName,
		"clusterID":      ctx.ClusterID,
		"clusterVersion": ctx.ClusterVersion,
		"userID":         ctx.UserID,

		"cliVersion":        config.VersionString,
		"os":                runtime.GOOS,
		"arch":              runtime.GOARCH,
		"machine_id":        get().MachineID,
		"measurementSource": "cli",
		"triggerSource":     config.GetDeployOrigin(),
		"isAgent":           agent != "",
		"isCI":              ci,
		"isAutomation":      ci || agent != "",
	}
	if agent != "" {
		props["agentType"] = agent
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
