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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/posthog/posthog-go"
	"github.com/stretchr/testify/require"
)

type fakePostHogRequest struct {
	method string
	path   string
	body   []byte
}

type fakePostHogServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []fakePostHogRequest
}

func newFakePostHogServer(t *testing.T) *fakePostHogServer {
	t.Helper()
	f := &fakePostHogServer{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		f.mu.Lock()
		f.requests = append(f.requests, fakePostHogRequest{method: r.Method, path: r.URL.Path, body: body})
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":1}`))
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakePostHogServer) received() []fakePostHogRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakePostHogRequest(nil), f.requests...)
}

type fakePostHogBatch struct {
	APIKey string `json:"api_key"`
	Batch  []struct {
		Timestamp  time.Time      `json:"timestamp"`
		Properties map[string]any `json:"properties"`
		DistinctID string         `json:"distinct_id"`
		Event      string         `json:"event"`
		UUID       string         `json:"uuid"`
	} `json:"batch"`
}

func newRealPostHogBackend(t *testing.T, endpoint string) *posthogBackend {
	t.Helper()
	client, err := posthog.NewWithConfig("phc_test_key", posthog.Config{
		Endpoint: endpoint,
	})
	require.NoError(t, err)
	return &posthogBackend{client: client}
}

func TestPostHogBackend_RealClientDeliversEventOnClose(t *testing.T) {
	srv := newFakePostHogServer(t)
	b := newRealPostHogBackend(t, srv.URL)

	b.enqueue(context.Background(), "user-123", posthogUpStartedEvent, posthog.Properties{
		"service":     "api",
		"workflow_id": "wf-1",
	})
	b.Close()

	requests := srv.received()
	require.Len(t, requests, 1)
	require.Equal(t, http.MethodPost, requests[0].method)
	require.Equal(t, "/batch/", requests[0].path)

	var got fakePostHogBatch
	require.NoError(t, json.Unmarshal(requests[0].body, &got))
	require.Equal(t, "phc_test_key", got.APIKey)
	require.Len(t, got.Batch, 1)
	require.Equal(t, posthogUpStartedEvent, got.Batch[0].Event)
	require.Equal(t, "user-123", got.Batch[0].DistinctID)
	require.Equal(t, "api", got.Batch[0].Properties["service"])
	require.Equal(t, "wf-1", got.Batch[0].Properties["workflow_id"])
	require.NotEmpty(t, got.Batch[0].UUID)
	require.False(t, got.Batch[0].Timestamp.IsZero())
	require.WithinDuration(t, time.Now(), got.Batch[0].Timestamp, time.Minute)
}

func TestPostHogBackend_RealClientBatchesMultipleEvents(t *testing.T) {
	srv := newFakePostHogServer(t)
	b := newRealPostHogBackend(t, srv.URL)

	b.enqueue(context.Background(), "user-1", posthogUpEvent, posthog.Properties{"n": "1"})
	b.enqueue(context.Background(), "user-2", posthogDeployPipelineTriggeredEvent, posthog.Properties{"n": "2"})
	b.Close()

	events := map[string]string{}
	for _, req := range srv.received() {
		var got fakePostHogBatch
		require.NoError(t, json.Unmarshal(req.body, &got))
		for _, e := range got.Batch {
			events[e.Event] = e.DistinctID
		}
	}
	require.Equal(t, map[string]string{
		posthogUpEvent:                      "user-1",
		posthogDeployPipelineTriggeredEvent: "user-2",
	}, events)
}

func TestPostHogBackend_RealClientCloseWithoutEvents(t *testing.T) {
	srv := newFakePostHogServer(t)
	b := newRealPostHogBackend(t, srv.URL)

	b.Close()

	require.Empty(t, srv.received())
}
