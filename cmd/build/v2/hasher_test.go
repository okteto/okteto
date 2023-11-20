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

package v2

import (
	"errors"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
)

type fakeRepositoryCommitRetriever struct {
	err error
	sha string
}

func (frcr fakeRepositoryCommitRetriever) GetSHA() (string, error) {
	return frcr.sha, frcr.err
}

func (frcr fakeRepositoryCommitRetriever) GetTreeHash(string) (string, error) {
	return frcr.sha, frcr.err
}

func TestServiceHasher_HashProjectCommit(t *testing.T) {
	tests := []struct {
		name         string
		repoCtrl     repositoryCommitRetriever
		expectedHash string
	}{
		{
			name: "success",
			repoCtrl: fakeRepositoryCommitRetriever{
				sha: "testsha",
			},
			expectedHash: "351af1ac3fe4b4f0a5550abe35e962b379b44e03b29349eb2dbf75677426393f",
		},
		{
			name: "error",
			repoCtrl: fakeRepositoryCommitRetriever{
				err: errors.New("fake error"),
			},
			expectedHash: "59fd822c5d8b629f00198fcca6e12053cbdc5a59675d60401dfee4f571a65538",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sh := newServiceHasher(tt.repoCtrl)
			hash := sh.HashProjectCommit(&model.BuildInfo{})
			assert.Equal(t, tt.expectedHash, hash)
		})
	}
}

func TestServiceHasher_HashBuildContext(t *testing.T) {
	tests := []struct {
		name         string
		repoCtrl     repositoryCommitRetriever
		expectedHash string
	}{
		{
			name: "success",
			repoCtrl: fakeRepositoryCommitRetriever{
				sha: "testtreehash",
			},
			expectedHash: "9e17f6a3555569e8f63413fc86cb1eb5daedafd3f1b4702cfc82d9a22951b4bc",
		},
		{
			name: "error",
			repoCtrl: fakeRepositoryCommitRetriever{
				err: errors.New("fake error"),
			},
			expectedHash: "624cba95f4e2bf43aa05717d71a18d6657beb7d366defcd59e8dc7da225bbdb2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sh := newServiceHasher(tt.repoCtrl)
			hash := sh.HashBuildContext(&model.BuildInfo{})
			assert.Equal(t, tt.expectedHash, hash)
		})
	}
}

func TestServiceHasher_HashService(t *testing.T) {
	tests := []struct {
		repoCtrl     repositoryCommitRetriever
		expectedErr  error
		name         string
		expectedHash string
	}{
		{
			name: "use_build_context",
			repoCtrl: fakeRepositoryCommitRetriever{
				sha: "testtreehash",
			},
			expectedHash: "ae8c29eca94ae08edaba704a0db10c8453951ca6879c87e0ea5d4daa7f5bb3b6",
		},
		{
			name: "use_project_commit",
			repoCtrl: fakeRepositoryCommitRetriever{
				sha: "testsha",
			},
			expectedHash: "351af1ac3fe4b4f0a5550abe35e962b379b44e03b29349eb2dbf75677426393f",
		},
		{
			name: "error",
			repoCtrl: fakeRepositoryCommitRetriever{
				err: errors.New("fake error"),
			},
			expectedHash: "59fd822c5d8b629f00198fcca6e12053cbdc5a59675d60401dfee4f571a65538",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sh := newServiceHasher(tt.repoCtrl)
			hash := sh.HashService(&model.BuildInfo{})
			assert.Equal(t, tt.expectedHash, hash)
		})
	}
}
