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
	"sync"

	"golang.org/x/sync/singleflight"
)

// previewResolver memoizes, per namespace, whether the namespace is a preview
// environment, so the analytics preview check hits the Okteto API at most once
// per session even though it is evaluated from several command call sites
// (up wake, deploy, pipeline) during a single okteto up.
type previewResolver struct {
	cache sync.Map // namespace name → bool (is within preview)
	sg    singleflight.Group
}

// isWithinPreview returns whether ns is a preview environment, evaluated via
// checkPreview and cached by namespace name. Unlike UIDResolver, the boolean
// result IS cached even when checkPreview reports "not a preview" (a non-nil
// error), because that is the common case and caching it is what deduplicates
// the dominant path. Errors are collapsed to false (best-effort analytics) and
// not retried; concurrent callers of the same namespace collapse into a single
// checkPreview call via singleflight.
func (r *previewResolver) isWithinPreview(ctx context.Context, ns string, checkPreview func(context.Context, string) error) bool {
	if v, ok := r.cache.Load(ns); ok {
		return v.(bool)
	}

	v, _, _ := r.sg.Do(ns, func() (any, error) {
		if v, ok := r.cache.Load(ns); ok {
			return v.(bool), nil
		}
		result := checkPreview(ctx, ns) == nil
		r.cache.Store(ns, result)
		return result, nil
	})
	return v.(bool)
}

// defaultPreviewResolver is the process-wide resolver shared by IsWithinPreview
// across all command call sites, mirroring how currentAnalytics is a package
// singleton in config.go.
var defaultPreviewResolver = &previewResolver{}
