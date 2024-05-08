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

package syncthing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestGetFiles(t *testing.T) {
	dir := t.TempDir()
	defer func() {
		os.Unsetenv(constants.OktetoFolderEnvVar)
	}()

	t.Setenv(constants.OktetoFolderEnvVar, dir)
	log := GetLogFile("test", "application")
	expected := filepath.Join(dir, "test", "application", "syncthing.log")

	if log != expected {
		t.Errorf("got %s, expected %s", log, expected)
	}

	info := getInfoFile("test", "application")
	expected = filepath.Join(dir, "test", "application", "syncthing.info")
	if info != expected {
		t.Errorf("got %s, expected %s", info, expected)
	}
}

func TestIdentifyReadinessIssue(t *testing.T) {
	var tests = []struct {
		expectedErr error
		mockFs      func() afero.Fs
		name        string
		s           Syncthing
	}{
		{
			name: "no-matching-errors",
			s: Syncthing{
				LogPath: "syncthing.log",
			},
			mockFs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "syncthing.log", []byte("happy days!"), 0644)
				return fs
			},
		},
		{
			name: "matching-insufficient-space-for-database",
			s: Syncthing{
				LogPath: "syncthing.log",
			},
			mockFs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "syncthing.log", []byte("2024/01/01 10:00:00 INFO: Failed initial scan of sendonly folder \"1\" (okteto-1)"+
					"2024/01/01 10:01:00 WARNING: Error on folder \"1\" (okteto-1): insufficient space on disk for database (/home/<user>/.okteto/<namespace>/<service>/index-v0.14.0.db): current 0.94 % < required 1 %"+
					"2024/01/01 10:02:00 failed to sufficiently increase receive buffer size (was: 208 kiB, wanted: 2048 kiB, got: 416 kiB). See https://github.com/quic-go/quic-go/wiki/UDP-Buffer-Sizes for details."), 0644)
				return fs
			},
			expectedErr: oktetoErrors.ErrInsufficientSpaceOnUserDisk,
		},
		{
			name: "matching-error-opening-database",
			s: Syncthing{
				LogPath: "syncthing.log",
			},
			mockFs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "syncthing.log", []byte("[start] \"2024/01/01 10:01:00 WARNING: Error opening database: mkdir /home/<user>/.okteto/<namespace>/<service>/index-v0.14.0.db: no space left on device"), 0644)
				return fs
			},
			expectedErr: oktetoErrors.ErrInsufficientSpaceOnUserDisk,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.s.Fs = tt.mockFs()
			err := tt.s.IdentifyReadinessIssue()
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
