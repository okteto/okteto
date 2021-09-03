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

package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	dir, err := ioutil.TempDir("", t.Name())
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dir)

	from := filepath.Join(dir, "from")
	to := filepath.Join(dir, "to")

	if err := CopyFile(from, to); err == nil {
		t.Error("failed to return error for missing file")
	}

	content := []byte("hello-world")
	if err := ioutil.WriteFile(from, content, 0600); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(from, to); err != nil {
		t.Fatalf("failed to copy from %s to %s: %s", from, to, err)
	}

	copied, err := ioutil.ReadFile(to)
	if err != nil {
		t.Fatal(err)
	}

	if string(content) != string(copied) {
		t.Fatalf("got %s, expected %s", string(content), string(copied))
	}

	if err := CopyFile(from, to); err != nil {
		t.Fatalf("failed to overwrite from %s to %s: %s", from, to, err)
	}

}

func TestFileExists(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dir)

	p := filepath.Join(dir, "exists")
	if FileExists(p) {
		t.Errorf("fail to detect non-existing file")
	}

	if err := ioutil.WriteFile(p, []byte("hello-world"), 0600); err != nil {
		t.Fatal(err)
	}

	if !FileExists(p) {
		t.Errorf("fail to detect existing file")
	}
}

func Test_GetValidNameFromFolder(t *testing.T) {
	var tests = []struct {
		name     string
		folder   string
		expected string
	}{
		{name: "all lower case", folder: "lowercase", expected: "lowercase"},
		{name: "with some lower case", folder: "lowerCase", expected: "lowercase"},
		{name: "upper case", folder: "UpperCase", expected: "uppercase"},
		{name: "valid symbols", folder: "getting-started.test", expected: "getting-started-test"},
		{name: "invalid symbols", folder: "getting_$#started", expected: "getting-started"},
		{name: "current folder", folder: ".", expected: "model"},
		{name: "parent folder", folder: "..", expected: "pkg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := GetValidNameFromFolder(tt.folder)
			if err != nil {
				t.Errorf("got an error in '%s': %s", tt.name, err)
			}
			if actual != tt.expected {
				t.Errorf("'%s' got '%s' expected '%s'", tt.name, actual, tt.expected)
			}
		})
	}
}

func Test_GetValidNameFromGitRepo(t *testing.T) {
	var tests = []struct {
		name     string
		gitRepo  string
		expected string
	}{
		{name: "https url", gitRepo: "https://github.com/okteto/stacks-getting-started", expected: "stacks-getting-started"},
		{name: "https with slash at the end", gitRepo: "https://github.com/okteto/stacks-getting-started/", expected: "stacks-getting-started"},
		{name: "ssh url", gitRepo: "git@github.com:okteto/stacks-getting-started.git", expected: "stacks-getting-started"},
		{name: "ssh url with slash at the end", gitRepo: "git@github.com:okteto/stacks-getting-started.git/", expected: "stacks-getting-started"},
		{name: "https with dots", gitRepo: "https://github.com/okteto/stacks.getting.started", expected: "stacks-getting-started"},
		{name: "URL with uppers", gitRepo: "https://github.com/okteto/StacksGettingStarted", expected: "stacksgettingstarted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateURLToName(tt.gitRepo)

			if result != tt.expected {
				t.Errorf("'%s' got '%s' expected '%s'", tt.name, result, tt.expected)
			}
		})
	}

}
