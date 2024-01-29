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

package analytics

import (
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/okteto"
)

func TestMain(m *testing.M) {
	currentAnalytics = &Analytics{
		Enabled:   false,
		MachineID: "machine-id",
	}
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {
				Name:      "test",
				Namespace: "namespace",
				UserID:    "user-id",
			},
		},
	}
	os.Exit(m.Run())
}

func Test_generatedMachineID(t *testing.T) {
	m := generateMachineID()
	if m == "" || m == "na" {
		t.Fatalf("bad machineID: %s", m)
	}

	for i := 0; i < 10; i++ {
		nM := generateMachineID()
		if nM != m {
			t.Fatalf("different machineID.\ngot: %s\nexpected: %s", nM, m)
		}
	}
}

func Test_getTrackID(t *testing.T) {
	var tests = []struct {
		name      string
		machineID string
		userID    string
	}{
		{name: "new", machineID: ""},
		{name: "existing-machine-id", machineID: "machine-123"},
		{name: "existing-user-id", machineID: "machine-123", userID: "user-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			t.Setenv(constants.OktetoHomeEnvVar, dir)

			a := get()
			a.MachineID = tt.machineID
			okteto.GetContext().UserID = tt.userID

			trackID := getTrackID()

			expected := ""
			switch {
			case len(tt.userID) > 0:
				expected = tt.userID
			case len(tt.machineID) > 0:
				expected = tt.machineID
			}

			if trackID != expected {
				t.Fatalf("different trackID.\ngot: %s\nexpected: %s", trackID, expected)
			}
		})
	}
}

func Test_disabledInOktetoCluster(t *testing.T) {
	var tests = []struct {
		contextStore *okteto.ContextStore
		name         string
		expected     bool
	}{
		{
			name: "cloud-always-enabled",
			contextStore: &okteto.ContextStore{
				Contexts:       map[string]*okteto.Context{okteto.CloudURL: {Name: okteto.CloudURL, IsOkteto: true, Analytics: false}},
				CurrentContext: okteto.CloudURL,
			},
			expected: false,
		},
		{
			name: "vanilla-always-enabled",
			contextStore: &okteto.ContextStore{
				Contexts:       map[string]*okteto.Context{"minikube": {Name: "minikube", IsOkteto: false, Analytics: true}},
				CurrentContext: "minikube",
			},
			expected: false,
		},
		{
			name: "admin-enabled",
			contextStore: &okteto.ContextStore{
				Contexts:       map[string]*okteto.Context{"oe": {Name: "oe", IsOkteto: true, Analytics: true}},
				CurrentContext: "oe",
			},
			expected: false,
		},
		{
			name: "admin-disabled",
			contextStore: &okteto.ContextStore{
				Contexts:       map[string]*okteto.Context{"oe": {Name: "oe", IsOkteto: true, Analytics: false}},
				CurrentContext: "oe",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = tt.contextStore
			result := disabledByOktetoAdmin()
			if result != tt.expected {
				t.Fatalf("test %s, expected %t result %t", tt.name, tt.expected, result)
			}
		})
	}
}
