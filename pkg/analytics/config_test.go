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

package analytics

import (
	"testing"

	"github.com/okteto/okteto/pkg/constants"
)

func Test_Get(t *testing.T) {
	var tests = []struct {
		name             string
		currentAnalytics bool
		enabled          bool
		fileExits        bool
		want             bool
	}{
		{
			name:             "is-currentAnalytics-enabled",
			currentAnalytics: true,
			enabled:          true,
			want:             true,
		},
		{
			name:             "is-currentAnalytics-disabled",
			currentAnalytics: true,
			enabled:          false,
			want:             false,
		},
		{
			name:      "is-currentAnalytics-nil-file-not-exists",
			fileExits: false,
			want:      false,
		},
		{
			name:      "is-currentAnalytics-nil-file-exists-enabled",
			fileExits: true,
			enabled:   true,
			want:      true,
		},
		{
			name:      "is-currentAnalytics-nil-file-exists-disabled",
			fileExits: true,
			enabled:   false,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			t.Setenv(constants.OktetoFolderEnvVar, dir)

			if !tt.currentAnalytics {
				currentAnalytics = nil
			} else {
				currentAnalytics = &Analytics{Enabled: tt.enabled}
			}

			if tt.fileExits {
				a := &Analytics{Enabled: tt.enabled}
				if err := a.save(); err != nil {
					t.Fatalf("analytics file wasn't created")
				}
			}

			if got := get().Enabled; got != tt.want {
				t.Errorf("After Init, got %v, want %v", got, tt.want)
			}

		})
	}

}
