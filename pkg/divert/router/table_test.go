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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// testLogger records everything written to it so tests can assert on skipped entries.
type testLogger struct {
	messages []string
}

func (l *testLogger) Infof(format string, args ...interface{}) {
	l.messages = append(l.messages, fmt.Sprintf(format, args...))
}

func TestStaticTableLookup(t *testing.T) {
	const service = "api"

	tests := []struct {
		name              string
		spec              string
		lookupService     string
		lookupKey         string
		expectedNamespace string
		expectedOk        bool
	}{
		{
			name:          "empty spec never resolves",
			spec:          "",
			lookupService: service,
			lookupKey:     "alice",
			expectedOk:    false,
		},
		{
			name:              "single route",
			spec:              "alice:alice-dev",
			lookupService:     service,
			lookupKey:         "alice",
			expectedNamespace: "alice-dev",
			expectedOk:        true,
		},
		{
			name:              "one of several routes",
			spec:              "alice:alice-dev,bob:bob-dev",
			lookupService:     service,
			lookupKey:         "bob",
			expectedNamespace: "bob-dev",
			expectedOk:        true,
		},
		{
			name:              "whitespace around entries is trimmed",
			spec:              "  alice : alice-dev ,  bob : bob-dev  ",
			lookupService:     service,
			lookupKey:         "alice",
			expectedNamespace: "alice-dev",
			expectedOk:        true,
		},
		{
			name:          "unknown key does not resolve",
			spec:          "alice:alice-dev",
			lookupService: service,
			lookupKey:     "carol",
			expectedOk:    false,
		},
		{
			name:          "another service does not resolve",
			spec:          "alice:alice-dev",
			lookupService: "catalog",
			lookupKey:     "alice",
			expectedOk:    false,
		},
		{
			name:              "unparseable entry does not discard the valid ones",
			spec:              "brokenentry,alice:alice-dev",
			lookupService:     service,
			lookupKey:         "alice",
			expectedNamespace: "alice-dev",
			expectedOk:        true,
		},
		{
			name:          "entry without a key is skipped",
			spec:          ":alice-dev",
			lookupService: service,
			lookupKey:     "",
			expectedOk:    false,
		},
		{
			name:          "entry without a namespace is skipped",
			spec:          "alice:",
			lookupService: service,
			lookupKey:     "alice",
			expectedOk:    false,
		},
		{
			name:              "first entry wins over a duplicate key",
			spec:              "alice:alice-dev,alice:someone-else",
			lookupService:     service,
			lookupKey:         "alice",
			expectedNamespace: "alice-dev",
			expectedOk:        true,
		},
		{
			name:              "trailing separator is ignored",
			spec:              "alice:alice-dev,",
			lookupService:     service,
			lookupKey:         "alice",
			expectedNamespace: "alice-dev",
			expectedOk:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := NewStaticTable(service, tt.spec, &testLogger{})

			namespace, ok := table.Lookup(tt.lookupService, tt.lookupKey)

			require.Equal(t, tt.expectedOk, ok)
			require.Equal(t, tt.expectedNamespace, namespace)
		})
	}
}

func TestNewStaticTable_ReportsUnparseableEntries(t *testing.T) {
	logger := &testLogger{}

	NewStaticTable("api", "brokenentry,alice:alice-dev", logger)

	require.Len(t, logger.messages, 1)
	require.Contains(t, logger.messages[0], "brokenentry")
}

func TestNewStaticTable_ReportsDuplicateKeys(t *testing.T) {
	logger := &testLogger{}

	NewStaticTable("api", "alice:alice-dev,alice:someone-else", logger)

	require.Len(t, logger.messages, 1)
	require.Contains(t, logger.messages[0], "alice-dev")
}

func TestNewStaticTable_ValidSpecIsSilent(t *testing.T) {
	logger := &testLogger{}

	NewStaticTable("api", "alice:alice-dev,bob:bob-dev", logger)

	require.Empty(t, logger.messages)
}
