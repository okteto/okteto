// Copyright 2021 The Okteto Authors
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

package pipeline

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/okteto/okteto/pkg/model"
)

func Test_getRepositoryURL(t *testing.T) {

	type remote struct {
		name string
		url  string
	}
	var tests = []struct {
		name        string
		expectError bool
		remotes     []remote
		expect      string
	}{
		{
			name:        "single origin",
			expectError: false,
			remotes: []remote{
				{name: "origin", url: "https://github.com/okteto/go-getting-started"},
			},
			expect: "https://github.com/okteto/go-getting-started",
		},
		{
			name:        "single remote",
			expectError: false,
			remotes: []remote{
				{name: "mine", url: "https://github.com/okteto/go-getting-started"},
			},
			expect: "https://github.com/okteto/go-getting-started",
		},
		{
			name:        "multiple remotes",
			expectError: false,
			remotes: []remote{
				{name: "fork", url: "https://github.com/oktetotest/go-getting-started"},
				{name: "origin", url: "https://github.com/cindy/go-getting-started"},
				{name: "upstream", url: "https://github.com/okteto/go-getting-started"},
			},
			expect: "https://github.com/cindy/go-getting-started",
		},
		{
			name:        "no remotes",
			expectError: true,
			remotes:     nil,
			expect:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			if _, err := model.GetRepositoryURL(dir); err == nil {

				t.Fatal("expected error when there's no github repo")
			}

			r, err := git.PlainInit(dir, true)
			if err != nil {
				t.Fatal(err)
			}

			for _, rm := range tt.remotes {
				if _, err := r.CreateRemote(&config.RemoteConfig{Name: rm.name, URLs: []string{rm.url}}); err != nil {
					t.Fatal(err)
				}
			}

			url, err := model.GetRepositoryURL(dir)

			if tt.expectError {
				if err == nil {
					t.Error("expected error when calling getRepositoryURL")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if url != tt.expect {
				t.Errorf("expected '%s', got '%s", tt.expect, url)
			}
		})
	}
}
