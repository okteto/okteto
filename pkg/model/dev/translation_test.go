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

package dev

import (
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model/build"
	"github.com/okteto/okteto/pkg/model/constants"
	"github.com/okteto/okteto/pkg/model/environment"
)

func TestSSHServerPortTranslationRule(t *testing.T) {
	tests := []struct {
		name     string
		manifest *Dev
		expected environment.Environment
	}{
		{
			name: "default",
			manifest: &Dev{
				Image:         &build.Build{},
				SSHServerPort: constants.OktetoDefaultSSHServerPort,
			},
			expected: environment.Environment{
				{Name: "OKTETO_NAMESPACE", Value: ""},
				{Name: "OKTETO_NAME", Value: ""},
			},
		},
		{
			name: "custom port",
			manifest: &Dev{
				Image:         &build.Build{},
				SSHServerPort: 22220,
			},
			expected: environment.Environment{
				{Name: "OKTETO_NAMESPACE", Value: ""},
				{Name: "OKTETO_NAME", Value: ""},
				{Name: constants.OktetoSSHServerPortVariableEnvVar, Value: "22220"},
			},
		},
	}
	for _, test := range tests {
		t.Logf("test: %s", test.name)
		rule := test.manifest.ToTranslationRule(test.manifest, false)
		if e, a := test.expected, rule.Environment; !reflect.DeepEqual(e, a) {
			t.Errorf("expected environment:\n%#v\ngot:\n%#v", e, a)
		}
	}
}
