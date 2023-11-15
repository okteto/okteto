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

package cmd

import (
	"fmt"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestGetDevFromArgs(t *testing.T) {
	tests := []struct {
		expectedErr   error
		manifest      *model.Manifest
		expectedDev   *model.Dev
		name          string
		args          []string
		activeDevMode []string
	}{
		{
			name: "first arg is on dev section",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"autocreate": &model.Dev{},
				},
			},
			args:        []string{"autocreate", "sh"},
			expectedDev: &model.Dev{},
			expectedErr: nil,
		},
		{
			name: "only one argument/same name as dev",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"sh": &model.Dev{},
				},
			},
			args:        []string{"sh"},
			expectedDev: &model.Dev{},
			expectedErr: nil,
		},
		{
			name: "several argument/first argument same name as dev",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"sh": &model.Dev{},
				},
			},
			args:        []string{"sh", "autocreate"},
			expectedDev: &model.Dev{},
			expectedErr: nil,
		},
		{
			name: "dev is not found ",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"api":       &model.Dev{},
					"other-dev": &model.Dev{},
				},
			},
			args:        []string{"not-api", "autocreate"},
			expectedDev: nil,
			expectedErr: fmt.Errorf(oktetoErrors.ErrDevContainerNotExists, "not-api"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, err := getDevFromArgs(tt.manifest, tt.args, tt.activeDevMode)
			assert.Equal(t, tt.expectedDev, dev)
			if err != nil {
				assert.Error(t, err, tt.expectedErr.Error())
			}
		})
	}
}

func TestGetCommandFromArgs(t *testing.T) {
	tests := []struct {
		name         string
		manifest     *model.Manifest
		args         []string
		expectedArgs []string
	}{
		{
			name: "first arg is on dev section",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"autocreate": &model.Dev{},
				},
			},
			args:         []string{"autocreate", "sh"},
			expectedArgs: []string{"sh"},
		},
		{
			name: "only one argument/same name as dev",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"sh": &model.Dev{},
				},
			},
			args:         []string{"sh"},
			expectedArgs: []string{"sh"},
		},
		{
			name: "several argument/first argument same name as dev",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"sh": &model.Dev{},
				},
			},
			args:         []string{"sh", "autocreate"},
			expectedArgs: []string{"autocreate"},
		},
		{
			name: "dev is not found ",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"api":       &model.Dev{},
					"other-dev": &model.Dev{},
				},
			},
			args:         []string{"not-api", "autocreate"},
			expectedArgs: []string{"not-api", "autocreate"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev := getCommandToRunFromArgs(tt.manifest, tt.args)
			assert.Equal(t, tt.expectedArgs, dev)
		})
	}
}
