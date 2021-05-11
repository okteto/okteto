package pipeline

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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

			if _, err := GetRepositoryURL(context.TODO(), dir); err == nil {
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

			url, err := GetRepositoryURL(context.TODO(), dir)
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

func Test_getBranch(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	r, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	_, err = GetBranch(context.TODO(), dir)
	if err == nil {
		t.Fatal("expected no-branch error")
	}

	filename := filepath.Join(dir, "example-git-file")
	if err := ioutil.WriteFile(filename, []byte("hello world!"), 0644); err != nil {
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

	b, err := GetBranch(context.TODO(), dir)
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

	if _, err := GetBranch(context.TODO(), dir); err == nil {
		t.Fatal("didn't fail when getting a non branch")
	}
}
