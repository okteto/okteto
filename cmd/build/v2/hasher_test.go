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
			hash := sh.hashProjectCommit(&model.BuildInfo{})
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
			expectedHash: "71e012767fbae26c772d966c13505eb79f093829daa6210887b5ad13a666a74c",
		},
		{
			name: "error",
			repoCtrl: fakeRepositoryCommitRetriever{
				err: errors.New("fake error"),
			},
			expectedHash: "13c6e75356db55df9032754f1e4cc9ca58b2ccb7392d636c3e6313896a8acba7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sh := newServiceHasher(tt.repoCtrl)
			hash := sh.hashBuildContext(&model.BuildInfo{})
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
			hash := sh.hashService(&model.BuildInfo{})
			assert.Equal(t, tt.expectedHash, hash)
		})
	}
}

func TestGetCommitHash(t *testing.T) {

	sh := serviceHasher{
		buildContextCache: map[string]string{
			"test-context": "cached-commit-hash",
		},
		projectCommit: "project-commit-hash",
	}
	testCases := []struct {
		name               string
		oktetoEnvValue     string
		buildContext       string
		expectedCommitHash string
	}{
		{
			name:               "OktetoSmartBuildUsingContextEnvVar true, missing build context",
			oktetoEnvValue:     "true",
			buildContext:       "",
			expectedCommitHash: "",
		},
		{
			name:               "OktetoSmartBuildUsingContextEnvVar false",
			oktetoEnvValue:     "false",
			buildContext:       "test-context",
			expectedCommitHash: "project-commit-hash",
		},
		{
			name:               "OktetoSmartBuildUsingContextEnvVar true, build context found in cache",
			oktetoEnvValue:     "true",
			buildContext:       "test-context",
			expectedCommitHash: "cached-commit-hash",
		},
	}

	// Iterate over test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(OktetoSmartBuildUsingContextEnvVar, tc.oktetoEnvValue)

			buildInfo := &model.BuildInfo{
				Context: tc.buildContext,
			}

			result := sh.GetCommitHash(buildInfo)

			assert.Equal(t, tc.expectedCommitHash, result)
		})
	}
}
