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

package ssh

import (
	"testing"
	"time"
)

func TestParseSSHTimeoutRequiresPositiveDuration(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "invalid", "0", "0s", "-1s"} {
		if _, err := parseSSHTimeout(value); err == nil {
			t.Fatalf("parseSSHTimeout(%q) accepted a non-positive or malformed timeout", value)
		}
	}

	got, err := parseSSHTimeout("30s")
	if err != nil {
		t.Fatal(err)
	}
	if got != 30*time.Second {
		t.Fatalf("parseSSHTimeout(30s) = %s, want 30s", got)
	}
}
