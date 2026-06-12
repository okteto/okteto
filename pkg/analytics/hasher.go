// Copyright 2023 The Okteto Authors
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
	"crypto/sha256"
	"encoding/hex"
	"strings"

	giturls "github.com/chainguard-dev/git-urls"
)

func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// normalizeRepoURL normalizes a git repo URL before hashing:
// parses all git URL formats (https, ssh://, git@), forces https scheme,
// strips credentials, query params, fragments, trailing .git suffix, and lowercases the result.
func normalizeRepoURL(rawURL string) string {
	u, err := giturls.Parse(rawURL)
	if err != nil || u.Host == "" {
		return strings.ToLower(rawURL)
	}

	u.Scheme = "https"
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimRight(strings.TrimSuffix(strings.TrimRight(u.Path, "/"), ".git"), "/")

	return strings.ToLower(u.String())
}
