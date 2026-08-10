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
	"strings"
)

// Table resolves a routing key to the namespace the request must be diverted to.
//
// Phase 0 backs this with a static table built from an environment variable. Phase 1
// replaces it with a ConfigMap informer without the router itself changing: an
// implementation that returns ok=false always degrades to the baseline, which is the only
// contract the router depends on.
type Table interface {
	// Lookup returns the destination namespace for a (service, routing key) pair.
	// ok is false when the pair is unknown, which the router treats as "go to baseline".
	Lookup(service, key string) (namespace string, ok bool)
}

// StaticTable is a Table with a fixed set of routes, scoped to a single service.
type StaticTable struct {
	routes  map[string]string
	service string
}

// NewStaticTable builds a Table for service from a route spec of the form
// `key1:namespace1,key2:namespace2`.
//
// Unparseable entries are skipped and reported through logger rather than failing: a router
// that refuses to start because one developer wrote a bad entry takes the shared service
// down for everyone. An empty spec is valid and yields a table that always falls back to
// the baseline.
func NewStaticTable(service, spec string, logger Logger) *StaticTable {
	return &StaticTable{
		service: service,
		routes:  ParseRoutes(spec, logger),
	}
}

// ParseRoutes decodes a route spec of the form `key1:namespace1,key2:namespace2` into a map
// of routing key to destination namespace.
//
// It is exported so that anything reading a router's configuration back out of the cluster
// decodes it the same way the router itself does.
func ParseRoutes(spec string, logger Logger) map[string]string {
	routes := map[string]string{}

	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		key, namespace, found := strings.Cut(entry, ":")
		key = strings.TrimSpace(key)
		namespace = strings.TrimSpace(namespace)
		if !found || key == "" || namespace == "" {
			logger.Infof("skipping unparseable route entry %q: expected format 'key:namespace'", entry)
			continue
		}

		if previous, duplicated := routes[key]; duplicated {
			logger.Infof("skipping duplicate route entry %q: key already routes to namespace %q", entry, previous)
			continue
		}

		routes[key] = namespace
	}

	return routes
}

// Lookup implements Table.
func (t *StaticTable) Lookup(service, key string) (string, bool) {
	if service != t.service {
		return "", false
	}

	namespace, ok := t.routes[key]
	return namespace, ok
}
