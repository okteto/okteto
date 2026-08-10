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

package router

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const filePermissions = 0o600

// routeDir builds a directory shaped like a mounted ConfigMap: one file per routing key,
// named after the key, containing the destination namespace.
func routeDir(t *testing.T, routes map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for key, namespace := range routes {
		writeRoute(t, dir, key, namespace)
	}

	return dir
}

func writeRoute(t *testing.T, dir, key, namespace string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, key), []byte(namespace), filePermissions))
}

func TestFileTable_ResolvesAKeyFromTheDirectory(t *testing.T) {
	table := NewFileTable(testService, routeDir(t, map[string]string{"alice": "alice-dev"}), &testLogger{})

	namespace, ok := table.Lookup(testService, "alice")

	require.True(t, ok)
	require.Equal(t, "alice-dev", namespace)
}

// Several developers sharing one router is the whole reason the table lives in a ConfigMap.
func TestFileTable_ResolvesSeveralDevelopersAtOnce(t *testing.T) {
	table := NewFileTable(testService, routeDir(t, map[string]string{"alice": "alice-dev", "bob": "bob-dev"}), &testLogger{})

	alice, aliceOK := table.Lookup(testService, "alice")
	bob, bobOK := table.Lookup(testService, "bob")

	require.True(t, aliceOK)
	require.Equal(t, "alice-dev", alice)
	require.True(t, bobOK)
	require.Equal(t, "bob-dev", bob)
}

func TestFileTable_UnknownKeyDoesNotResolve(t *testing.T) {
	table := NewFileTable(testService, routeDir(t, map[string]string{"alice": "alice-dev"}), &testLogger{})

	_, ok := table.Lookup(testService, "carol")

	require.False(t, ok)
}

func TestFileTable_AnotherServiceDoesNotResolve(t *testing.T) {
	table := NewFileTable(testService, routeDir(t, map[string]string{"alice": "alice-dev"}), &testLogger{})

	_, ok := table.Lookup("catalog", "alice")

	require.False(t, ok)
}

func TestFileTable_TrimsTheTrailingNewlineATextEditorLeaves(t *testing.T) {
	dir := t.TempDir()
	writeRoute(t, dir, "alice", "alice-dev\n")
	table := NewFileTable(testService, dir, &testLogger{})

	namespace, _ := table.Lookup(testService, "alice")

	require.Equal(t, "alice-dev", namespace)
}

// A missing directory must degrade to "no routes", not stop the router: every request then
// goes to the baseline, which is normal behaviour rather than an outage.
func TestFileTable_MissingDirectoryResolvesNothing(t *testing.T) {
	table := NewFileTable(testService, filepath.Join(t.TempDir(), "not-created-yet"), &testLogger{})

	_, ok := table.Lookup(testService, "alice")

	require.False(t, ok)
}

func TestFileTable_EmptyEntryIsSkipped(t *testing.T) {
	dir := t.TempDir()
	writeRoute(t, dir, "alice", "   ")
	table := NewFileTable(testService, dir, &testLogger{})

	_, ok := table.Lookup(testService, "alice")

	require.False(t, ok)
}

func TestFileTable_OneBadEntryDoesNotDiscardTheGoodOnes(t *testing.T) {
	dir := t.TempDir()
	writeRoute(t, dir, "alice", "alice-dev")
	writeRoute(t, dir, "bob", "")
	table := NewFileTable(testService, dir, &testLogger{})

	_, aliceOK := table.Lookup(testService, "alice")
	_, bobOK := table.Lookup(testService, "bob")

	require.True(t, aliceOK)
	require.False(t, bobOK)
}

// A mounted ConfigMap is not a plain directory: the kubelet keeps a `..data` symlink and a
// timestamped directory behind it alongside the real keys.
func TestFileTable_IgnoresTheKubeletsOwnBookkeeping(t *testing.T) {
	dir := t.TempDir()
	writeRoute(t, dir, "alice", "alice-dev")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "..2026_08_06_10_00_00.1234"), 0o700))
	writeRoute(t, dir, "..data", "not-a-route")

	table := NewFileTable(testService, dir, &testLogger{})

	_, dataOK := table.Lookup(testService, "..data")
	_, aliceOK := table.Lookup(testService, "alice")
	require.False(t, dataOK)
	require.True(t, aliceOK)
}

func TestFileTable_ReloadPicksUpADeveloperJoining(t *testing.T) {
	dir := routeDir(t, map[string]string{"alice": "alice-dev"})
	table := NewFileTable(testService, dir, &testLogger{})
	writeRoute(t, dir, "bob", "bob-dev")

	table.Reload()

	namespace, ok := table.Lookup(testService, "bob")
	require.True(t, ok)
	require.Equal(t, "bob-dev", namespace)
}

func TestFileTable_ReloadPicksUpADeveloperLeaving(t *testing.T) {
	dir := routeDir(t, map[string]string{"alice": "alice-dev", "bob": "bob-dev"})
	table := NewFileTable(testService, dir, &testLogger{})
	require.NoError(t, os.Remove(filepath.Join(dir, "bob")))

	table.Reload()

	_, bobOK := table.Lookup(testService, "bob")
	_, aliceOK := table.Lookup(testService, "alice")
	require.False(t, bobOK)
	require.True(t, aliceOK)
}

// Losing the directory must not drop routes that are working: an empty table would send
// every diverted developer back to the baseline at once.
func TestFileTable_ReloadKeepsRoutesWhenTheDirectoryDisappears(t *testing.T) {
	dir := routeDir(t, map[string]string{"alice": "alice-dev"})
	table := NewFileTable(testService, dir, &testLogger{})
	require.NoError(t, os.RemoveAll(dir))

	table.Reload()

	_, ok := table.Lookup(testService, "alice")
	require.True(t, ok)
}

func TestFileTable_WatchAppliesChangesUntilCancelled(t *testing.T) {
	dir := routeDir(t, map[string]string{"alice": "alice-dev"})
	table := NewFileTable(testService, dir, &testLogger{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go table.Watch(ctx, time.Millisecond)
	writeRoute(t, dir, "bob", "bob-dev")

	require.Eventually(t, func() bool {
		_, ok := table.Lookup(testService, "bob")
		return ok
	}, time.Second, 5*time.Millisecond)
}

func TestFileTable_WatchStopsWithTheContext(t *testing.T) {
	table := NewFileTable(testService, routeDir(t, map[string]string{"alice": "alice-dev"}), &testLogger{})
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})

	go func() {
		table.Watch(ctx, time.Millisecond)
		close(stopped)
	}()
	cancel()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Watch did not return after the context was cancelled")
	}
}

// The router serves requests while the watcher reloads, so this must be race-free.
func TestFileTable_LookupIsSafeDuringAReload(t *testing.T) {
	dir := routeDir(t, map[string]string{"alice": "alice-dev"})
	table := NewFileTable(testService, dir, &testLogger{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})

	go table.Watch(ctx, time.Microsecond)
	go func() {
		for i := 0; i < 2000; i++ {
			table.Lookup(testService, "alice")
		}
		close(done)
	}()

	<-done
}

func TestFileTable_LogsOnlyWhenTheRoutesChange(t *testing.T) {
	logger := &testLogger{}
	table := NewFileTable(testService, routeDir(t, map[string]string{"alice": "alice-dev"}), logger)
	before := len(logger.messages)

	table.Reload()

	require.Len(t, logger.messages, before)
}

func TestDescribeRoutes(t *testing.T) {
	require.Equal(t, "no routes, everything goes to the baseline", describeRoutes(map[string]string{}))
	require.Equal(t, "alice=alice-dev bob=bob-dev", describeRoutes(map[string]string{"bob": "bob-dev", "alice": "alice-dev"}))
}
