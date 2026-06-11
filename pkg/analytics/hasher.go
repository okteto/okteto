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
	"net/url"
	"strings"
)

func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// normalizeRepoURL normalizes a git repo URL before hashing:
// converts SSH git@ format to HTTPS, forces https scheme,
// strips a trailing .git suffix and trailing slashes, and lowercases the result.
func normalizeRepoURL(rawURL string) string {
	if strings.HasPrefix(rawURL, "git@") {
		rawURL = "https://" + strings.Replace(strings.TrimPrefix(rawURL, "git@"), ":", "/", 1)
	}

	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return strings.ToLower(rawURL)
	}

	u.Scheme = "https"
	u.Path = strings.TrimRight(strings.TrimSuffix(strings.TrimRight(u.Path, "/"), ".git"), "/")

	return strings.ToLower(u.String())
}
