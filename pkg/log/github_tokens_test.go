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

package log

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The tokens are built at runtime so secret scanners don't flag the test file
var (
	classicToken      = "ghp_" + strings.Repeat("a1B2", 9)
	installationToken = "ghs_" + strings.Repeat("Z9y8", 9)
	statelessToken    = "ghs_123456_" + strings.Repeat("eyJh", 8) + "." + strings.Repeat("eyJz", 10) + "." + strings.Repeat("S1g-", 10)
	fineGrainedToken  = "github_pat_" + strings.Repeat("11AB", 8) + "_" + strings.Repeat("cd34", 14)
)

func TestMaskGitHubTokens(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "classic token",
			message:  "token: " + classicToken,
			expected: "token: ***",
		},
		{
			name:     "installation token in a clone URL",
			message:  "cloning https://x-access-token:" + installationToken + "@github.com/acme/infra.git",
			expected: "cloning https://x-access-token:***@github.com/acme/infra.git",
		},
		{
			name:     "stateless installation token with a JWT",
			message:  "password=" + statelessToken,
			expected: "password=***",
		},
		{
			name:     "fine-grained token",
			message:  fineGrainedToken + " is the token",
			expected: "*** is the token",
		},
		{
			name:     "period that ends a sentence stays",
			message:  "the token is " + installationToken + ".",
			expected: "the token is ***.",
		},
		{
			name:     "several tokens",
			message:  classicToken + " " + installationToken,
			expected: "*** ***",
		},
		{
			name:     "short word with a token prefix stays",
			message:  "ghp_short and ghs_ are prefixes",
			expected: "ghp_short and ghs_ are prefixes",
		},
		{
			name:     "no token",
			message:  "git clone https://github.com/acme/infra",
			expected: "git clone https://github.com/acme/infra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, mask(tt.message, nil))
		})
	}
}

func TestRedactMessage_MasksGitHubTokensWhenMaskingIsOn(t *testing.T) {
	log.maskedWords = []string{}
	EnableMasking()
	t.Cleanup(DisableMasking)

	require.Equal(t, "token *** and secret", redactMessage("token "+classicToken+" and secret"))
}

func TestRedactMessage_KeepsGitHubTokensWhenMaskingIsOff(t *testing.T) {
	DisableMasking()

	require.Equal(t, "token "+classicToken, redactMessage("token "+classicToken))
}
