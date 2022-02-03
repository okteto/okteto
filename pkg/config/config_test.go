// Copyright 2022 The Okteto Authors
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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/model"
)

func TestGetUserHomeDir(t *testing.T) {
	home := GetUserHomeDir()
	if home == "" {
		t.Fatal("got an empty home value")
	}

	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.RemoveAll(dir)
		os.Unsetenv(model.OktetoHomeEnvVar)
	}()

	os.Setenv(model.OktetoHomeEnvVar, dir)
	home = GetUserHomeDir()
	if home != dir {
		t.Fatalf("OKTETO_HOME override failed, got %s instead of %s", home, dir)
	}

	oktetoHome := GetOktetoHome()
	expected := filepath.Join(dir, ".okteto")
	if oktetoHome != expected {
		t.Errorf("got %s, expected %s", oktetoHome, expected)
	}
}

func Test_homedirWindows(t *testing.T) {
	var tests = []struct {
		name     string
		expected string
		env      map[string]string
	}{
		{
			name:     "home",
			expected: `c:/users/okteto`,
			env: map[string]string{
				"HOME": `c:/users/okteto`,
			},
		},
		{
			name:     "USERPROFILE",
			expected: `c:\users\okteto`,
			env: map[string]string{
				"HOME": `c:\users\okteto`,
			},
		},
		{
			name:     "homepath",
			expected: `H:\okteto`,
			env: map[string]string{
				"HOMEDRIVE": `H:`,
				"HOMEPATH":  `\okteto`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := os.Getenv(model.HomeEnvVar)
			up := os.Getenv(model.UserProfileEnvVar)
			hp := os.Getenv(model.HomePathEnvVar)
			hd := os.Getenv(model.HomeDriveEnvVar)

			os.Unsetenv(model.HomeEnvVar)
			os.Unsetenv(model.UserProfileEnvVar)
			os.Unsetenv(model.HomePathEnvVar)
			os.Unsetenv(model.HomeDriveEnvVar)

			defer func() {
				os.Setenv(model.HomeEnvVar, home)
				os.Setenv(model.UserProfileEnvVar, up)
				os.Setenv(model.HomePathEnvVar, hp)
				os.Setenv(model.HomeDriveEnvVar, hd)
			}()

			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			got, err := homedirWindows()
			if err != nil {
				t.Error(err)
			}

			if got != tt.expected {
				t.Errorf("got %s, expected %s", got, tt.expected)
			}
		})
	}
}

func TestGetOktetoHome(t *testing.T) {
	dir, err := os.MkdirTemp("", t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.RemoveAll(dir)
		os.Unsetenv(model.OktetoFolderEnvVar)
	}()

	os.Setenv(model.OktetoFolderEnvVar, dir)

	got := GetOktetoHome()
	if got != dir {
		t.Errorf("expected %s, got %s", dir, got)
	}
}

func TestGetAppHome(t *testing.T) {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	os.Setenv(model.OktetoFolderEnvVar, dir)

	got := GetAppHome("ns", "dp")
	expected := filepath.Join(dir, "ns", "dp")
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
