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

package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
)

func Test_CheckIfDirectory(t *testing.T) {
	tests := []struct {
		want error
		name string
		path string
	}{
		{
			name: "directory",
			path: ".",
			want: nil,
		},
		{
			name: "file",
			path: "dev.go",
			want: fmt.Errorf("'dev.go' is not a directory"),
		},
		{
			name: "file",
			path: "no.go",
			want: fmt.Errorf("'no.go' does not exist"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckIfDirectory(tt.path)
			if got == nil && tt.want == nil {
				return
			}
			if got == nil || tt.want == nil {
				t.Errorf("CheckIfDirectory(%s) = %s, want %s", tt.path, got, tt.want)
			}
			if got.Error() != tt.want.Error() {
				t.Errorf("CheckIfDirectory(%s) = %s, want %s", tt.path, got, tt.want)
			}
		})
	}
}

func Test_GetDevFromManifest(t *testing.T) {
	wrongDevName := "not-test"
	tests := []struct {
		err      error
		manifest *model.Manifest
		dev      *model.Dev
		name     string
		devName  string
	}{
		{
			name:     "manifest has no dev section",
			manifest: &model.Manifest{},
			devName:  "",
			dev:      nil,
			err:      oktetoErrors.ErrManifestNoDevSection,
		},
		{
			name: "manifest has one dev section but not the one the user added",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"test": &model.Dev{},
				},
			},
			devName: wrongDevName,
			dev:     nil,
			err:     fmt.Errorf(oktetoErrors.ErrDevContainerNotExists, wrongDevName),
		},
		{
			name: "manifest has several dev section user introduces wrong one",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"test":   &model.Dev{},
					"test-2": &model.Dev{},
				},
			},
			devName: wrongDevName,
			dev:     nil,
			err:     fmt.Errorf(oktetoErrors.ErrDevContainerNotExists, wrongDevName),
		},
		{
			name: "manifest has several dev section user introduces correct one",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"test":   &model.Dev{},
					"test-2": &model.Dev{},
				},
			},
			devName: "test",
			dev:     &model.Dev{},
			err:     nil,
		},
		{
			name: "manifest has one dev section and user introduces empty devName",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"test": &model.Dev{},
				},
			},
			devName: "",
			dev:     &model.Dev{},
			err:     nil,
		},
		{
			name: "manifest has several dev section user introduces empty devName",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"test":   &model.Dev{},
					"test-2": &model.Dev{},
				},
			},
			devName: "",
			dev:     nil,
			err:     ErrNoDevSelected,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, err := GetDevFromManifest(tt.manifest, tt.devName)
			assert.Equal(t, tt.dev, dev)
			if tt.err != nil {
				assert.Equal(t, tt.err.Error(), err.Error())
			}
		})
	}
}

type FakeOktetoSelector struct {
	err error
	dev string
}

func (s *FakeOktetoSelector) AskForOptionsOkteto(_ []SelectorItem, _ int) (string, error) {
	return s.dev, s.err
}

func Test_SelectDevFromManifest(t *testing.T) {
	localAbsPath, err := filepath.Abs("/")
	assert.NoError(t, err)

	tests := []struct {
		err      error
		manifest *model.Manifest
		selector *FakeOktetoSelector
		dev      *model.Dev
		name     string
	}{
		{
			name: "dev-is-selected",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"test": &model.Dev{
						Name:            "test",
						ImagePullPolicy: "Always",
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  "/",
									RemotePath: "/remote",
								},
							},
						},
						SSHServerPort: 80,
						Image:         &build.Info{},
					},
					"test-2": &model.Dev{},
				},
				ManifestPath: filepath.Join(localAbsPath, "okteto.yml"),
			},
			selector: &FakeOktetoSelector{
				dev: "test",
			},
			dev: &model.Dev{
				Name:            "test",
				ImagePullPolicy: "Always",
				Sync: model.Sync{
					Folders: []model.SyncFolder{
						{
							LocalPath:  "/",
							RemotePath: "/remote",
						},
					},
				},
				SSHServerPort: 80,
				Image:         &build.Info{},
			},
		},
		{
			name: "dev-is-not-valid",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"test":   &model.Dev{},
					"test-2": &model.Dev{},
				},
			},
			selector: &FakeOktetoSelector{
				dev: "test",
			},
			err: fmt.Errorf("supported values for 'imagePullPolicy' are: 'Always', 'IfNotPresent' or 'Never'"),
		},
		{
			name:     "selector-returns-err",
			manifest: &model.Manifest{},
			selector: &FakeOktetoSelector{
				err: errors.New("error-from-selector"),
			},
			err: errors.New("error-from-selector"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, err := SelectDevFromManifest(tt.manifest, tt.selector, tt.manifest.Dev.GetDevs())
			assert.Equal(t, tt.dev, dev)
			if tt.err != nil {
				assert.Equal(t, tt.err.Error(), err.Error())
			}
		})
	}
}

func Test_AskYesNo(t *testing.T) {
	tests := []struct {
		name     string
		def      YesNoDefault
		answer   string
		expected bool
	}{
		{
			name:     "ignores-default-when-answer",
			def:      YesNoDefault_No,
			answer:   "y\n",
			expected: true,
		},
		{
			name:     "honors-default-when-no-answer",
			def:      YesNoDefault_Yes,
			answer:   "\n",
			expected: true,
		},
		{
			name:     "ignores-default-when-answer",
			def:      YesNoDefault_No,
			answer:   "Y\n",
			expected: true,
		},
		{
			name:     "ignores-default-when-answer",
			def:      YesNoDefault_No,
			answer:   "N\n",
			expected: false,
		},
	}
	for _, tt := range tests {
		// Create a temp dir for files used to mock stdin
		dir := t.TempDir()
		t.Run(tt.name, func(t *testing.T) {
			tmpPath := filepath.Join(dir, fmt.Sprintf("yes_no_test-%s", tt.name))
			tmpFile, err := os.Create(tmpPath)
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				if err := tmpFile.Close(); err != nil {
					t.Fatal(err)
				}

				os.Remove(tmpFile.Name())
			}()

			if _, err := tmpFile.WriteString(tt.answer); err != nil {
				t.Fatal(err)
			}

			if _, err := tmpFile.Seek(0, 0); err != nil {
				t.Fatal(err)
			}

			oldStdin := os.Stdin
			defer func() { os.Stdin = oldStdin }()

			os.Stdin = tmpFile

			got, err := AskYesNo("", tt.def)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}

}
