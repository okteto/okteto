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

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
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
