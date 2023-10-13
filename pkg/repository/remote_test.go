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

func TestRemoteIsCleanDirTrue(t *testing.T) {
	remote := oktetoRemoteServiceController{
		isClean: true,
	}
	isClean, err := remote.isCleanDir(context.Background(), "")
	assert.NoError(t, err)
	assert.True(t, isClean)
}

func TestRemoteIsCleanDirFalse(t *testing.T) {
	remote := oktetoRemoteServiceController{
		isClean: false,
	}
	isClean, err := remote.isCleanDir(context.Background(), "")
	assert.NoError(t, err)
	assert.False(t, isClean)
}

func TestRemoteGetBuildHash(t *testing.T) {
	remote := oktetoRemoteServiceController{
		hash: "abcd100",
	}
	sha, err := remote.getHashByDir("")
	assert.NoError(t, err)
	assert.Equal(t, remote.hash, sha)
}
