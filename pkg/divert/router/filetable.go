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
	"sort"
	"strings"
	"sync"
	"time"
)

// DefaultReloadInterval is how often the router re-reads its route directory. The kubelet's
// own ConfigMap sync is the dominant delay, so polling faster than this buys nothing.
const DefaultReloadInterval = 5 * time.Second

// FileTable is a Table backed by a directory holding one file per routing key, named after
// the key and containing the destination namespace. That is exactly how Kubernetes
// materialises a mounted ConfigMap, so the route table can be updated without the router
// having any access to the API — no ServiceAccount, no Role, no RoleBinding.
//
// One file per key rather than one file listing them all: two developers joining the same
// divert then write different ConfigMap keys and cannot clobber each other.
//
// A future informer-backed table is a drop-in replacement: the router only depends on the
// Table interface, and on unknown keys resolving to "not found" so traffic falls back to
// the baseline.
type FileTable struct {
	routes  map[string]string
	logger  Logger
	service string
	dir     string
	mu      sync.RWMutex
}

// NewFileTable returns a table reading from dir, populated with whatever is there now.
//
// It never fails. A directory that cannot be read yields an empty table, which sends every
// request to the baseline — the same thing an unknown routing key does. Refusing to start
// would take the shared service down over a route table problem.
func NewFileTable(service, dir string, logger Logger) *FileTable {
	t := &FileTable{
		service: service,
		dir:     dir,
		logger:  logger,
		routes:  map[string]string{},
	}
	t.Reload()

	return t
}

// Lookup implements Table.
func (t *FileTable) Lookup(service, key string) (string, bool) {
	if service != t.service {
		return "", false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	namespace, ok := t.routes[key]
	return namespace, ok
}

// Watch re-reads the directory until the context is cancelled. The kubelet updates a mounted
// ConfigMap in place, so there is no event to subscribe to from inside the container.
func (t *FileTable) Watch(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.Reload()
		}
	}
}

// Reload replaces the in-memory routes with what is on disk. Individually unreadable or
// empty entries are skipped rather than failing the whole reload: one bad route must not
// take the others down with it.
func (t *FileTable) Reload() {
	entries, err := os.ReadDir(t.dir)
	if err != nil {
		// Expected before the volume is mounted, and whenever the ConfigMap does not exist
		// yet. Keep whatever was already loaded rather than dropping live routes.
		t.logger.Infof("could not read the route directory %q, keeping the routes already loaded: %s", t.dir, err)
		return
	}

	routes := make(map[string]string, len(entries))
	for _, entry := range entries {
		// A mounted ConfigMap contains the kubelet's own bookkeeping alongside the keys:
		// a `..data` symlink and a timestamped directory behind it.
		if strings.HasPrefix(entry.Name(), ".") || entry.IsDir() {
			continue
		}

		namespace, err := t.readRoute(entry.Name())
		if err != nil {
			t.logger.Infof("skipping route %q: %s", entry.Name(), err)
			continue
		}
		if namespace == "" {
			t.logger.Infof("skipping route %q: it names no destination namespace", entry.Name())
			continue
		}

		routes[entry.Name()] = namespace
	}

	t.swap(routes)
}

func (t *FileTable) readRoute(key string) (string, error) {
	contents, err := os.ReadFile(filepath.Join(t.dir, key))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(contents)), nil
}

// swap installs a new route set and reports what changed, since "which keys does the router
// currently know about" is the first question when a divert is not working.
func (t *FileTable) swap(routes map[string]string) {
	t.mu.Lock()
	changed := !sameRoutes(t.routes, routes)
	t.routes = routes
	t.mu.Unlock()

	if changed {
		t.logger.Infof("route table for service %q reloaded: %s", t.service, describeRoutes(routes))
	}
}

func sameRoutes(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for key, value := range a {
		if b[key] != value {
			return false
		}
	}

	return true
}

func describeRoutes(routes map[string]string) string {
	if len(routes) == 0 {
		return "no routes, everything goes to the baseline"
	}

	pairs := make([]string, 0, len(routes))
	for key, namespace := range routes {
		pairs = append(pairs, key+"="+namespace)
	}
	// Sorted so log lines stay stable across reloads and can be diffed by eye.
	sort.Strings(pairs)

	return strings.Join(pairs, " ")
}
