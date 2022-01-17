// Copyright 2021 The Okteto Authors
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

package secrets

import (
	"fmt"
	"log"
	"os"
	"testing"

	yaml "gopkg.in/yaml.v2"
)

func TestSecretMashalling(t *testing.T) {
	file, err := os.CreateTemp("/tmp", "okteto-secret-test")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(file.Name())

	if err := os.Setenv("TEST_HOME", file.Name()); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		data          string
		expected      *Secret
		expectedError bool
	}{
		{
			"local:remote",
			fmt.Sprintf("%s:/remote", file.Name()),
			&Secret{LocalPath: file.Name(), RemotePath: "/remote", Mode: 420},
			false,
		},
		{
			"local:remote:mode",
			fmt.Sprintf("%s:/remote:400", file.Name()),
			&Secret{LocalPath: file.Name(), RemotePath: "/remote", Mode: 256},
			false,
		},
		{
			"variables",
			"$TEST_HOME:/remote",
			&Secret{LocalPath: file.Name(), RemotePath: "/remote", Mode: 420},
			false,
		},
		{
			"too-short",
			"local",
			nil,
			true,
		},
		{
			"too-long",
			"local:remote:mode:other",
			nil,
			true,
		},
		{
			"wrong-local",
			"/local:/remote:400",
			nil,
			true,
		},
		{
			"wrong-remote",
			fmt.Sprintf("%s:remote", file.Name()),
			nil,
			true,
		},
		{
			"wrong-mode",
			fmt.Sprintf("%s:/remote:aaa", file.Name()),
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Secret
			if err := yaml.Unmarshal([]byte(tt.data), &result); err != nil {
				if !tt.expectedError {
					t.Fatalf("unexpected error unmarshaling %s: %s", tt.name, err.Error())
				}
				return
			}
			if tt.expectedError {
				t.Fatalf("expected error unmarshaling %s not thrown", tt.name)
			}
			if result.LocalPath != tt.expected.LocalPath {
				t.Errorf("didn't unmarshal correctly LocalPath. Actual %s, Expected %s", result.LocalPath, tt.expected.LocalPath)
			}
			if result.RemotePath != tt.expected.RemotePath {
				t.Errorf("didn't unmarshal correctly RemotePath. Actual %s, Expected %s", result.RemotePath, tt.expected.RemotePath)
			}
			if result.Mode != tt.expected.Mode {
				t.Errorf("didn't unmarshal correctly Mode. Actual %d, Expected %d", result.Mode, tt.expected.Mode)
			}

			_, err := yaml.Marshal(&result)
			if err != nil {
				t.Fatalf("error marshaling %s: %s", tt.name, err)
			}
		})
	}
}
