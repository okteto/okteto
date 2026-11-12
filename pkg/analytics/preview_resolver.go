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
	"os"
	"sync"

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"golang.org/x/sync/singleflight"
)

// IsWithinPreview reports whether the current CLI context is inside a preview
// environment. It first checks the platform env var (set when running inside a
// preview); if not set, it calls checkPreview with the active namespace so that
// `okteto ns use <preview-name>` is also detected. Pass nil to skip the API check.
// It feeds Mixpanel's Deploy.isPreview and the is_within_preview product event property.
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
	// Memoized per namespace so the preview API check runs at most once per
	// session across the up/deploy/pipeline call sites.
	return defaultPreviewResolver.isWithinPreview(ctx, ns, checkPreview)
}

// previewResolver memoizes, per namespace, whether it is a preview environment,
// so the analytics preview check hits the Okteto API at most once per session
// across the up/deploy/pipeline call sites.
type previewResolver struct {
	cache sync.Map // namespace name → bool (is within preview)
	sg    singleflight.Group
}

// isWithinPreview reports whether ns is a preview, via checkPreview, cached by
// namespace. Only definitive outcomes are cached: a preview (nil error) or a
// not-found error ("not a preview", the common case). A transient error (e.g. a
// timeout) returns false but is NOT cached, so a later call can retry instead of
// being stuck with a poisoned false. Concurrent callers for the same namespace
// collapse into one checkPreview via singleflight.
func (r *previewResolver) isWithinPreview(ctx context.Context, ns string, checkPreview func(context.Context, string) error) bool {
	if v, ok := r.cache.Load(ns); ok {
		return v.(bool)
	}

	v, _, _ := r.sg.Do(ns, func() (any, error) {
		if v, ok := r.cache.Load(ns); ok {
			return v.(bool), nil
		}
		err := checkPreview(ctx, ns)
		if err != nil && !oktetoErrors.IsNotFound(err) {
			// Transient error: don't cache it, allow a later call to retry.
			return false, nil
		}
		result := err == nil
		r.cache.Store(ns, result)
		return result, nil
	})
	return v.(bool)
}

// defaultPreviewResolver is the process-wide resolver shared by IsWithinPreview,
// mirroring the currentAnalytics package singleton in config.go.
var defaultPreviewResolver = &previewResolver{}
