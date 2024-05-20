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

package smartbuild

import (
	"errors"
	"testing"

	"github.com/okteto/okteto/pkg/build"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestServiceHasher_HashProjectCommit(t *testing.T) {
	fakeErr := errors.New("fake error")
	tests := []struct {
		repoCtrl     repositoryCommitRetriever
		expectedErr  error
		name         string
		expectedHash string
	}{
		{
			name: "success",
			repoCtrl: fakeConfigRepo{
				sha: "testsha",
			},
			expectedHash: "832d66070268d5a47860e9bd4402f504a1c0fe8d0c2dc1ecf814af610de72f0e",
			expectedErr:  nil,
		},
		{
			name: "error",
			repoCtrl: fakeConfigRepo{
				err: fakeErr,
			},
			expectedHash: "",
			expectedErr:  fakeErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sh := newServiceHasher(tt.repoCtrl, afero.NewMemMapFs())
			hash, err := sh.hashProjectCommit(&build.Info{})
			assert.Equal(t, tt.expectedHash, hash)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestServiceHasher_HashBuildContext(t *testing.T) {
	fakeErr := errors.New("fake error")
	serviceName := "fake-service"
	tests := []struct {
		repoCtrl     repositoryCommitRetriever
		name         string
		expectedHash string
	}{
		{
			name: "success",
			repoCtrl: fakeConfigRepo{
				sha: "testtreehash",
			},
			expectedHash: "b6f9d71cf55933c6e385102e196522fd73c279c1edaa919b565706fb0bc3d8ce",
		},
		{
			name: "error",
			repoCtrl: fakeConfigRepo{
				err: fakeErr,
			},
			expectedHash: "11cf9a064fe9b8419441515afbc33d22257ce11269ecbea211575f8cf4a33fac",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sh := &serviceHasher{
				gitRepoCtrl: tt.repoCtrl,
				fs:          afero.NewMemMapFs(),
				getCurrentTimestampNano: func() int64 {
					return int64(12312345252)
				},
				serviceShaCache: map[string]string{},
			}
			hash := sh.hashWithBuildContext(&build.Info{}, serviceName)
			assert.Equal(t, tt.expectedHash, hash)
		})
	}
}

func TestGetBuildContextHashInCache(t *testing.T) {
	tests := []struct {
		name           string
		buildContext   string
		cacheValue     string
		expectedResult string
	}{
		{
			name:           "Cache Hit",
			buildContext:   "test",
			cacheValue:     "hash123",
			expectedResult: "hash123",
		},
		{
			name:           "Cache Miss",
			buildContext:   "nonexistentBuildContext",
			cacheValue:     "",
			expectedResult: "",
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sh := newServiceHasher(nil, afero.NewMemMapFs())
			sh.serviceShaCache["test"] = tt.cacheValue
			result := sh.getServiceShaInCache(tt.buildContext)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
