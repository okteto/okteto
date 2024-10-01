// Copyright 2024 The Okteto Authors
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

package validator

import (
	"testing"

	okErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestFileArgumentIsNotDir(t *testing.T) {

	type args struct {
		fs         afero.Fs
		file       string
		dir        string
		createFile bool
	}
	tests := []struct {
		wantErr error
		name    string
		args    args
	}{
		{
			name: "empty file",
			args: args{
				fs:   afero.NewMemMapFs(),
				file: "",
			},
			wantErr: nil,
		},
		{
			name: "file not found",
			args: args{
				fs:   afero.NewMemMapFs(),
				file: "file",
			},
			wantErr: okErrors.ErrManifestPathNotFound,
		},
		{
			name: "file is a directory",
			args: args{
				fs:   afero.NewMemMapFs(),
				file: "dir",
				dir:  "dir",
			},
			wantErr: okErrors.ErrManifestPathIsDir,
		},
		{
			name: "file is a file",
			args: args{
				fs:         afero.NewMemMapFs(),
				file:       "file",
				createFile: true,
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.args.dir != "" {
				err := tt.args.fs.Mkdir(tt.args.dir, 0755)
				assert.NoError(t, err)
			}
			if tt.args.createFile {
				_, err := tt.args.fs.Create(tt.args.file)
				assert.NoError(t, err)
			}

			err := FileArgumentIsNotDir(tt.args.fs, tt.args.file)

			assert.Equal(t, tt.wantErr, err)
		})
	}
}
