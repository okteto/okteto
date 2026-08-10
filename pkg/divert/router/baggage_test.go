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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBaggageValue(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		key      string
		expected string
	}{
		{
			name:     "empty header",
			header:   "",
			key:      divertBaggageKey,
			expected: "",
		},
		{
			name:     "empty key",
			header:   "divert=alice",
			key:      "",
			expected: "",
		},
		{
			name:     "single member",
			header:   "divert=alice",
			key:      divertBaggageKey,
			expected: "alice",
		},
		{
			name:     "surrounding whitespace is trimmed",
			header:   "  divert  =  alice  ",
			key:      divertBaggageKey,
			expected: "alice",
		},
		{
			name:     "properties after semicolon are not part of the value",
			header:   "divert=alice;ttl=60;secure",
			key:      divertBaggageKey,
			expected: "alice",
		},
		{
			name:     "member among others",
			header:   "userId=42, divert=alice, sessionId=abc",
			key:      divertBaggageKey,
			expected: "alice",
		},
		{
			name:     "key not present",
			header:   "userId=42,sessionId=abc",
			key:      divertBaggageKey,
			expected: "",
		},
		{
			name:     "percent encoded value is decoded",
			header:   "divert=alice%2Fdev",
			key:      divertBaggageKey,
			expected: "alice/dev",
		},
		{
			name:     "plus is a literal plus, not a space",
			header:   "divert=alice+dev",
			key:      divertBaggageKey,
			expected: "alice+dev",
		},
		{
			name:     "invalid percent encoding falls back to the raw value",
			header:   "divert=%zz",
			key:      divertBaggageKey,
			expected: "%zz",
		},
		{
			name:     "member without a value separator is skipped",
			header:   "divert",
			key:      divertBaggageKey,
			expected: "",
		},
		{
			name:     "empty value",
			header:   "divert=",
			key:      divertBaggageKey,
			expected: "",
		},
		{
			name:     "first occurrence wins over a later duplicate",
			header:   "divert=alice,divert=bob",
			key:      divertBaggageKey,
			expected: "alice",
		},
		{
			name:     "keys are case sensitive",
			header:   "Divert=alice",
			key:      divertBaggageKey,
			expected: "",
		},
		{
			name:     "malformed member does not stop later members from being read",
			header:   "brokenmember,divert=alice",
			key:      divertBaggageKey,
			expected: "alice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, baggageValue(tt.header, tt.key))
		})
	}
}

func TestBaggageValue_ReadsTheLastMemberWithinTheLimit(t *testing.T) {
	header := paddingMembers(maxBaggageMembers-1) + ",divert=alice"

	require.Equal(t, "alice", baggageValue(header, divertBaggageKey))
}

func TestBaggageValue_IgnoresMembersPastTheLimit(t *testing.T) {
	header := paddingMembers(maxBaggageMembers) + ",divert=alice"

	require.Equal(t, "", baggageValue(header, divertBaggageKey))
}

// paddingMembers builds n valid baggage members that are not the divert key.
func paddingMembers(n int) string {
	members := make([]string, n)
	for i := range members {
		members[i] = fmt.Sprintf("pad%d=x", i)
	}
	return strings.Join(members, ",")
}
