// Copyright 2020 The Okteto Authors
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
	"io/ioutil"
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/model"
)

func Test_isWatchesConfigurationTooLow(t *testing.T) {
	var tests = []struct {
		name     string
		value    string
		expected bool
	}{
		{
			name:     "too-low",
			value:    "2",
			expected: true,
		},
		{
			name:     "too-low-trim",
			value:    "2\n",
			expected: true,
		},
		{
			name:     "ok",
			value:    "20000",
			expected: false,
		},
		{
			name:     "ok-trim",
			value:    "20000\n",
			expected: false,
		},
		{
			name:     "wrong",
			value:    "2a4d",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isWatchesConfigurationTooLow(tt.value)
			if tt.expected != result {
				t.Errorf("expected %t got %t in test %s", tt.expected, result, tt.name)
			}
		})
	}
}

func Test_loadDevOrDefault(t *testing.T) {
	name := "demo-deployment"
	d, err := loadDevOrDefault("/tmp/bad-path", name)
	if err != nil {
		t.Fatal("default dev was not returned")
	}

	if d.Name != name {
		t.Errorf("expected %s, got %s", name, d.Name)
	}

	d, err = loadDevOrDefault("/tmp/bad-path", "")
	if err == nil {
		t.Error("expected error with empty deployment name")
	}

	f, err := ioutil.TempFile("", "")
	f.Close()
	defer os.Remove(f.Name())

	existing := &model.Dev{
		Name:  name,
		Image: "okteto/test:1.0",
	}

	if err := saveManifest(existing, f.Name()); err != nil {
		t.Fatal(err)
	}

	d, err = loadDevOrDefault(f.Name(), "foo")
	if err != nil {
		t.Fatal("expected error with empty deployment name")
	}

	if existing.Image != d.Image {
		t.Fatalf("expected %s got %s", existing.Image, d.Image)
	}

	if existing.Name != d.Name {
		t.Fatalf("expected %s got %s", existing.Name, d.Name)
	}
}
