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

package filesystem

import "github.com/spf13/afero"

type FakeTemporalDirectoryCtrl struct {
	fs  afero.Fs
	err error
}

func NewTemporalDirectoryCtrl(fs afero.Fs) *FakeTemporalDirectoryCtrl {
	return &FakeTemporalDirectoryCtrl{
		fs:  fs,
		err: nil,
	}
}

func (fwd *FakeTemporalDirectoryCtrl) SetError(err error) {
	fwd.err = err
}

func (fwd FakeTemporalDirectoryCtrl) Create() (string, error) {
	dir, err := afero.TempDir(fwd.fs, "", "")
	if err != nil {
		return "", err
	}
	return dir, fwd.err
}
