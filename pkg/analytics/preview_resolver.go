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

// previewResolver memoizes, per namespace, whether it is a preview environment,
// so the analytics preview check hits the Okteto API at most once per session
// across the up/deploy/pipeline call sites.
type previewResolver struct {
	cache sync.Map // namespace name → bool (is within preview)
	sg    singleflight.Group
}

// isWithinPreview reports whether ns is a preview, via checkPreview, cached by
// namespace. The bool is cached even for "not a preview" (a non-nil error) — the
// common case — so errors collapse to false and are not retried. Concurrent
// callers for the same namespace collapse into one checkPreview via singleflight.
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

// defaultPreviewResolver is the process-wide resolver shared by IsWithinPreview,
// mirroring the currentAnalytics package singleton in config.go.
var defaultPreviewResolver = &previewResolver{}
