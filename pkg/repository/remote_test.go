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

package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoteIsCleanTrue(t *testing.T) {
	remote := oktetoRemoteRepoController{
		gitCommit: "123",
	}
	isClean, err := remote.isClean(context.Background())
	assert.NoError(t, err)
	assert.True(t, isClean)
}

func TestRemoteIsCleanFalse(t *testing.T) {
	remote := oktetoRemoteRepoController{
		gitCommit: "",
	}
	isClean, err := remote.isClean(context.Background())
	assert.NoError(t, err)
	assert.False(t, isClean)
}

func TestRemoteGetSHA(t *testing.T) {
	remote := oktetoRemoteRepoController{
		gitCommit: "123",
	}
	sha, err := remote.getSHA()
	assert.NoError(t, err)
	assert.Equal(t, remote.gitCommit, sha)
}

func TestRemoteGetLatestDirSHA(t *testing.T) {
	remote := oktetoRemoteRepoController{
		gitCommit: "123",
	}
	_, err := remote.GetLatestDirSHA("test")
	assert.Error(t, err, fmt.Errorf("not-implemented"))
}

func TestRemoteGetDiffHash(t *testing.T) {
	remote := oktetoRemoteRepoController{
		gitCommit: "123",
	}
	_, err := remote.GetDiffHash("test")
	assert.Error(t, err, fmt.Errorf("not-implemented"))
}
