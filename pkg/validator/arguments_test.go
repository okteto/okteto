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
		name    string
		args    args
		wantErr error
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
