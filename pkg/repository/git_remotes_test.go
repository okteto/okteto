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

//go:build !windows

package repository

import (
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/stretchr/testify/require"
)

func TestGetRepoURLWithoutOriginUsesTheFirstRemoteByName(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	for _, name := range []string{"upstream", "fork", "mirror"} {
		_, err = repo.CreateRemote(&config.RemoteConfig{Name: name, URLs: []string{"https://github.com/okteto/" + name + ".git"}})
		require.NoError(t, err)
	}

	// go-git returns the remotes in no particular order: every call has to pick the same one
	urls := map[string]int{}
	for range 30 {
		urls[NewRepository(dir).GetAnonymizedRepo()]++
	}

	require.Equal(t, map[string]int{"https://github.com/okteto/fork": 30}, urls)
}
