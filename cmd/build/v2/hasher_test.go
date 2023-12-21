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

	"github.com/okteto/okteto/pkg/build"
	"github.com/stretchr/testify/assert"
)

type fakeRepositoryCommitRetriever struct {
	err error
	sha string
}

func (frcr fakeRepositoryCommitRetriever) GetSHA() (string, error) {
	return frcr.sha, frcr.err
}

func (frcr fakeRepositoryCommitRetriever) GetLatestDirCommit(string) (string, error) {
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
			expectedHash: "0bfcfc438afefa7bb304fb26b3e9b261f6926aebb0a773d9f2b00935da99bf84",
		},
		{
			name: "error",
			repoCtrl: fakeRepositoryCommitRetriever{
				err: errors.New("fake error"),
			},
			expectedHash: "6b780261e68c89eadd9173079bd45df7a98da9adb2a847896618eccd44120293",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sh := newServiceHasher(tt.repoCtrl)
			hash := sh.hashProjectCommit(&build.BuildInfo{})
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
			expectedHash: "1a75474ba1006dcd56a8adceae64d47b4e82fe0a2fd99a4ef4ca51268105bf9b",
		},
		{
			name: "error",
			repoCtrl: fakeRepositoryCommitRetriever{
				err: errors.New("fake error"),
			},
			expectedHash: "6b780261e68c89eadd9173079bd45df7a98da9adb2a847896618eccd44120293",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sh := newServiceHasher(tt.repoCtrl)
			hash := sh.hashBuildContext(&build.BuildInfo{})
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
			expectedHash: "1a75474ba1006dcd56a8adceae64d47b4e82fe0a2fd99a4ef4ca51268105bf9b",
		},
		{
			name: "use_project_commit",
			repoCtrl: fakeRepositoryCommitRetriever{
				sha: "testsha",
			},
			expectedHash: "0bfcfc438afefa7bb304fb26b3e9b261f6926aebb0a773d9f2b00935da99bf84",
		},
		{
			name: "error",
			repoCtrl: fakeRepositoryCommitRetriever{
				err: errors.New("fake error"),
			},
			expectedHash: "6b780261e68c89eadd9173079bd45df7a98da9adb2a847896618eccd44120293",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sh := newServiceHasher(tt.repoCtrl)
			hash := sh.hashService(&build.BuildInfo{})
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

			buildInfo := &build.BuildInfo{
				Context: tc.buildContext,
			}

			result := sh.GetCommitHash(buildInfo)

			assert.Equal(t, tc.expectedCommitHash, result)
		})
	}
}
