// Copyright 2022 The Okteto Authors
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

package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
)

func Test_getBranch(t *testing.T) {
	dir := t.TempDir()

	r, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	_, err = GetBranch(dir)

	if err == nil {
		t.Fatal("expected no-branch error")
	}

	filename := filepath.Join(dir, "example-git-file")
	if err := os.WriteFile(filename, []byte("hello world!"), 0600); err != nil {
		t.Fatal(err)
	}

	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := w.Add("example-git-file"); err != nil {
		t.Fatal(err)
	}

	commit, err := w.Commit("example go-git commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	headRef, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}

	ref := plumbing.NewHashReference("refs/heads/my-branch", headRef.Hash())
	if err := r.Storer.SetReference(ref); err != nil {
		t.Fatal(err)
	}

	if err := w.Checkout(&git.CheckoutOptions{
		Branch: ref.Name(),
	}); err != nil {
		t.Fatal(err)
	}

	b, err := GetBranch(dir)

	if err != nil {
		t.Fatal(err)
	}

	if b != "my-branch" {
		t.Errorf("expected branch my-branch, got %s", b)
	}

	if err := w.Checkout(&git.CheckoutOptions{
		Hash: commit,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := GetBranch(dir); err == nil {

		t.Fatal("didn't fail when getting a non branch")
	}
}

func Test_isOktetoRepoFromURL(t *testing.T) {
	var tests = []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "ssh from okteto",
			url:      "git@github.com:okteto/movies.git",
			expected: true,
		},
		{
			name:     "ssh from okteto",
			url:      "git@github.com:test/test.git",
			expected: false,
		},
		{
			name:     "https from okteto",
			url:      "https://github.com/okteto/test.git",
			expected: true,
		},
		{
			name:     "https from okteto",
			url:      "https://github.com/test/test.git",
			expected: false,
		},
		{
			name:     "ssh from okteto",
			url:      "ssh://git@github.com/okteto/test",
			expected: true,
		},
		{
			name:     "https from okteto",
			url:      "ssh://git@github.com/test/test",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isOktetoSample := isOktetoRepoFromURL(tt.url)
			assert.Equal(t, tt.expected, isOktetoSample)
		})
	}
}

func TestGetRandomSHA(t *testing.T) {
	SHALen := 40
	defaultSHA := strings.Repeat("0", SHALen)
	sha := GetRandomSHA()
	assert.Len(t, sha, SHALen)
	assert.NotEqual(t, sha, defaultSHA)
	anotherSHA := GetRandomSHA()
	assert.Len(t, anotherSHA, SHALen)
	assert.NotEqual(t, anotherSHA, defaultSHA)

	assert.NotEqual(t, anotherSHA, sha)
}
