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
	"github.com/stretchr/testify/mock"
)

type fakeWorkingDirGetter struct {
	workingDir string
	err        error
}

func (f fakeWorkingDirGetter) Get() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.workingDir, nil
}

type mockConfigRepo struct {
	mock.Mock
}

func (m *mockConfigRepo) GetSHA() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}
func (m *mockConfigRepo) GetLatestDirSHA(dir string) (string, error) {
	args := m.Called(dir)
	return args.String(0), args.Error(1)
}
func (m *mockConfigRepo) GetDiffHash(dir string) (string, error) {
	args := m.Called(dir)
	return args.String(0), args.Error(1)
}

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
			wdGetter := fakeWorkingDirGetter{}
			sh := newServiceHasher(tt.repoCtrl, afero.NewMemMapFs(), wdGetter)
			hash, err := sh.hashProjectCommit(&build.Info{})
			assert.Equal(t, tt.expectedHash, hash)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestServiceHasher_HashBuildContextWithError(t *testing.T) {
	serviceName := "fake-service"
	tests := []struct {
		name                 string
		wdGetter             *fakeWorkingDirGetter
		context              string
		dirSHA               string
		errSHA               error
		diffHash             string
		errHash              error
		expectedBuildContext string
		expectedHash         string
	}{
		{
			name: "withErrorGettingShaAndDiffHash",
			wdGetter: &fakeWorkingDirGetter{
				workingDir: "/tmp/test-okteto/working-dir",
			},
			context:              "test",
			dirSHA:               "",
			errSHA:               assert.AnError,
			diffHash:             "",
			errHash:              assert.AnError,
			expectedBuildContext: "/tmp/test-okteto/working-dir/test",
			expectedHash:         "70d446809d8ec0a5c83cd8c74a24b757524e26c85ccd8f8101f40ed3f51275ea",
		},
		{
			name: "withErrorGettingSha",
			wdGetter: &fakeWorkingDirGetter{
				workingDir: "/tmp/test-okteto/working-dir",
			},
			context:              "test",
			dirSHA:               "",
			errSHA:               assert.AnError,
			diffHash:             "e8a0e7cc771c6947f0808ebaef3f86b2ae8d2cf1",
			errHash:              nil,
			expectedBuildContext: "/tmp/test-okteto/working-dir/test",
			expectedHash:         "52f2fa59c20ab2b747ba019f6835d62c0cf76712261966b859e34957c4804578",
		},
		{
			name: "withErrorGettingDiffHash",
			wdGetter: &fakeWorkingDirGetter{
				workingDir: "/tmp/test-okteto/working-dir",
			},
			context:              "test",
			dirSHA:               "cc46d77bac8ccb52a3972689c95b55ed300adf33",
			errSHA:               nil,
			diffHash:             "",
			errHash:              assert.AnError,
			expectedBuildContext: "/tmp/test-okteto/working-dir/test",
			expectedHash:         "52b47d89a34e4ed991d415b72b3359ebc1d3204bab998ef94f1bec390dc4f0e0",
		},
		{
			name: "withoutErrorWithRelativePathOnContext",
			wdGetter: &fakeWorkingDirGetter{
				workingDir: "/tmp/test-okteto/working-dir",
			},
			context:              "test/service-a",
			dirSHA:               "cc46d77bac8ccb52a3972689c95b55ed300adf33",
			errSHA:               nil,
			diffHash:             "e8a0e7cc771c6947f0808ebaef3f86b2ae8d2cf1",
			errHash:              nil,
			expectedBuildContext: "/tmp/test-okteto/working-dir/test/service-a",
			expectedHash:         "f01cf2257431831e32332e11d817d2c144fb8668fc73de0aec5b80f3b7379f0d",
		},
		{
			name: "withoutErrorWithAbsolutePathOnContext",
			wdGetter: &fakeWorkingDirGetter{
				workingDir: "/tmp/test-okteto/working-dir",
			},
			context:              "/tmp/test-case/absolute-path/test/service-a",
			dirSHA:               "cc46d77bac8ccb52a3972689c95b55ed300adf33",
			errSHA:               nil,
			diffHash:             "e8a0e7cc771c6947f0808ebaef3f86b2ae8d2cf1",
			errHash:              nil,
			expectedBuildContext: "/tmp/test-case/absolute-path/test/service-a",
			expectedHash:         "c5de9669d6ad49db99a4a6415e61a378f98eb756fed8e0dc37fdf876436fcac4",
		},
		{
			name: "withoutErrorWithEmptyContext",
			wdGetter: &fakeWorkingDirGetter{
				workingDir: "/tmp/test-okteto/working-dir",
			},
			context:              "",
			dirSHA:               "cc46d77bac8ccb52a3972689c95b55ed300adf33",
			errSHA:               nil,
			diffHash:             "e8a0e7cc771c6947f0808ebaef3f86b2ae8d2cf1",
			errHash:              nil,
			expectedBuildContext: "/tmp/test-okteto/working-dir",
			expectedHash:         "63facbf3f5dede6119682e69dc444fa154d066a8449524a91862f8ada0e96d0f",
		},
		{
			name: "withErrorGettingWorkingDirectoryWithRelativeContext",
			wdGetter: &fakeWorkingDirGetter{
				err: assert.AnError,
			},
			context:              "test/service-a",
			dirSHA:               "cc46d77bac8ccb52a3972689c95b55ed300adf33",
			errSHA:               nil,
			diffHash:             "e8a0e7cc771c6947f0808ebaef3f86b2ae8d2cf1",
			errHash:              nil,
			expectedBuildContext: "test/service-a",
			expectedHash:         "f01cf2257431831e32332e11d817d2c144fb8668fc73de0aec5b80f3b7379f0d",
		},
		{
			name: "withErrorGettingWorkingDirectoryWithAbsoluteContext",
			wdGetter: &fakeWorkingDirGetter{
				err: assert.AnError,
			},
			context:              "/tmp/test-okteto/working-dir",
			dirSHA:               "cc46d77bac8ccb52a3972689c95b55ed300adf33",
			errSHA:               nil,
			diffHash:             "e8a0e7cc771c6947f0808ebaef3f86b2ae8d2cf1",
			errHash:              nil,
			expectedBuildContext: "/tmp/test-okteto/working-dir",
			expectedHash:         "0eb548056aa376058d70623c8fd687d57493b1de7093f57102f70a68eeba86de",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoController := &mockConfigRepo{}

			buildInfo := &build.Info{
				Context: tt.context,
			}

			sh := &serviceHasher{
				gitRepoCtrl: repoController,
				fs:          afero.NewMemMapFs(),
				getCurrentTimestampNano: func() int64 {
					return int64(12312345252)
				},
				serviceShaCache: map[string]string{},
				wdGetter:        tt.wdGetter,
			}

			repoController.On("GetLatestDirSHA", tt.expectedBuildContext).Return(tt.dirSHA, tt.errSHA)

			repoController.On("GetDiffHash", tt.expectedBuildContext).Return(tt.diffHash, tt.errHash)

			hash := sh.hashWithBuildContext(buildInfo, serviceName)

			assert.Equal(t, tt.expectedHash, hash)

			repoController.AssertExpectations(t)
		})
	}
}
