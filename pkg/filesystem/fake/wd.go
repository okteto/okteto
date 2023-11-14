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

type FakeWorkingDirectoryCtrl struct {
	errors FakeWorkingDirectoryCtrlErrors
	wd     string
}

type FakeWorkingDirectoryCtrlErrors struct {
	Getter error
	Setter error
}

func NewFakeWorkingDirectoryCtrl(dir string) *FakeWorkingDirectoryCtrl {
	return &FakeWorkingDirectoryCtrl{
		wd:     dir,
		errors: FakeWorkingDirectoryCtrlErrors{},
	}
}

func (fwd *FakeWorkingDirectoryCtrl) SetErrors(errors FakeWorkingDirectoryCtrlErrors) {
	fwd.errors = errors
}

func (fwd FakeWorkingDirectoryCtrl) Get() (string, error) {
	return fwd.wd, fwd.errors.Getter
}

func (fwd *FakeWorkingDirectoryCtrl) Change(dir string) error {
	fwd.wd = dir
	return fwd.errors.Setter
}
