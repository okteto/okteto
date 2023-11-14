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

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestFakeTemporalDirectoryCtrl(t *testing.T) {
	tempDirCtrl := NewTemporalDirectoryCtrl(afero.NewMemMapFs())
	var tests = []struct {
		errors error
		name   string
	}{
		{
			name:   "create with error",
			errors: assert.AnError,
		},
		{
			name:   "create with no error",
			errors: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDirCtrl.SetError(tt.errors)
			_, err := tempDirCtrl.Create()
			assert.Equal(t, tt.errors, err)
		})
	}
}
