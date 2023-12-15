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

	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestServiceHasher_HashProjectCommit(t *testing.T) {
	fakeErr := errors.New("fake error")
	tests := []struct {
		name         string
		repoCtrl     repositoryCommitRetriever
		expectedHash string
		expectedErr  error
	}{
		{
			name: "success",
			repoCtrl: fakeConfigRepo{
				sha: "testsha",
			},
			expectedHash: "cf0ff0bb100ae8a121de62276a5004349dcd6b349ceaecb3ba75ac344152dbe0",
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
			hash, err := sh.hashProjectCommit(&model.BuildInfo{})
			assert.Equal(t, tt.expectedHash, hash)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestServiceHasher_HashBuildContext(t *testing.T) {
	fakeErr := errors.New("fake error")
	tests := []struct {
		name         string
		repoCtrl     repositoryCommitRetriever
		expectedHash string
		expectedErr  error
	}{
		{
			name: "success",
			repoCtrl: fakeConfigRepo{
				sha: "testtreehash",
			},
			expectedHash: "52d0cacde546dd525296ab1893ea73b08e3033538c235ef3ac0a451aa6810ef0",
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
			hash, err := sh.hashBuildContext(&model.BuildInfo{})
			assert.Equal(t, tt.expectedHash, hash)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
