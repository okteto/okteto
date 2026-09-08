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
	"errors"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// resetPreviewResolver swaps the package resolver for a fresh one for the
// duration of the test, keeping preview-check tests hermetic.
func resetPreviewResolver(t *testing.T) {
	t.Helper()
	prev := defaultPreviewResolver
	t.Cleanup(func() { defaultPreviewResolver = prev })
	defaultPreviewResolver = &previewResolver{}
}

func TestPreviewResolver_CachesTrue(t *testing.T) {
	r := &previewResolver{}
	var calls int32
	checker := func(context.Context, string) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}

	require.True(t, r.isWithinPreview(context.Background(), "ns", checker))
	require.True(t, r.isWithinPreview(context.Background(), "ns", checker))
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestPreviewResolver_CachesFalse(t *testing.T) {
	r := &previewResolver{}
	var calls int32
	checker := func(context.Context, string) error {
		atomic.AddInt32(&calls, 1)
		return errors.New("not a preview")
	}

	require.False(t, r.isWithinPreview(context.Background(), "ns", checker))
	require.False(t, r.isWithinPreview(context.Background(), "ns", checker))
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestPreviewResolver_DifferentNamespacesDoNotCollide(t *testing.T) {
	r := &previewResolver{}
	yes := func(context.Context, string) error { return nil }
	no := func(context.Context, string) error { return errors.New("not a preview") }

	require.True(t, r.isWithinPreview(context.Background(), "preview", yes))
	require.False(t, r.isWithinPreview(context.Background(), "regular", no))

	// Cached independently: the stored value wins over the passed checker.
	require.True(t, r.isWithinPreview(context.Background(), "preview", no))
	require.False(t, r.isWithinPreview(context.Background(), "regular", yes))
}

func TestPreviewResolver_ConcurrentCallsCollapse(t *testing.T) {
	r := &previewResolver{}
	var calls int32
	started := make(chan struct{})
	release := make(chan struct{})
	// Returns an error (namespace is not a preview): the collapse property under
	// test is that checkPreview runs exactly once, independent of the result.
	checker := func(context.Context, string) error {
		if atomic.AddInt32(&calls, 1) == 1 {
			close(started)
			<-release
		}
		return errors.New("not a preview")
	}

	const n = 8
	results := make(chan bool, n)
	ready := make(chan struct{}, n-1)

	// Leader blocks inside checker.
	go func() { results <- r.isWithinPreview(context.Background(), "ns", checker) }()

	<-started
	for i := 0; i < n-1; i++ {
		go func() {
			ready <- struct{}{}
			results <- r.isWithinPreview(context.Background(), "ns", checker)
		}()
	}

	// Wait until every follower has started, then yield so they park inside
	// singleflight before releasing the leader. The calls==1 assertion is the
	// real guarantee: a follower that had not yet collapsed would start a second
	// call and fail the test.
	for i := 0; i < n-1; i++ {
		<-ready
	}
	runtime.Gosched()
	close(release)

	for i := 0; i < n; i++ {
		require.False(t, <-results)
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}
