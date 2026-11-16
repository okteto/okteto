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

package gitcredential

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

const githubHost = "github.com"

var (
	errMissingPath       = fmt.Errorf("the request from Git has no repository path, set %s to true", useHTTPPathKey)
	errMissingRepository = errors.New("the path in the request from Git has no repository")
)

// request is the part of Git's credential request the helper needs.
// See https://git-scm.com/docs/git-credential#IOFMT
type request struct {
	protocol string
	host     string
	path     string
}

// readRequest reads Git's "key=value" lines until a blank line or EOF.
// Keys the helper doesn't need, such as capability[] or wwwauth[], are ignored.
func readRequest(r io.Reader) (request, error) {
	req := request{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			break
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch key {
		case "protocol":
			req.protocol = value
		case "host":
			req.host = value
		case "path":
			req.path = value
		}
	}
	if err := scanner.Err(); err != nil {
		return request{}, fmt.Errorf("failed to read the request from Git: %w", err)
	}
	return req, nil
}

// isGitHub returns true when the request is for https://github.com. A host with a port isn't.
func (r request) isGitHub() bool {
	return r.protocol == "https" && strings.EqualFold(r.host, githubHost)
}

// repository returns the request's repository as "owner/repo". It drops the
// ".git" suffix and anything after the repository, such as Git LFS's "/info/lfs".
func (r request) repository() (string, error) {
	if r.path == "" {
		return "", errMissingPath
	}
	parts := strings.Split(strings.Trim(r.path, "/"), "/")
	if len(parts) < 2 {
		return "", errMissingRepository
	}
	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")
	if owner == "" || repo == "" {
		return "", errMissingRepository
	}
	return fmt.Sprintf("%s/%s", owner, repo), nil
}
