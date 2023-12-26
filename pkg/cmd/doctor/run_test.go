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

package doctor

import (
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/model"
	"gopkg.in/yaml.v2"
)

func Test_generateManifestFile(t *testing.T) {
	var tests = []struct {
		dev  *model.Dev
		name string
	}{
		{
			name: "empty",
			dev:  nil,
		},
		{
			name: "basic",
			dev: &model.Dev{
				Name:    "dev",
				Image:   &build.Info{Name: "okteto/dev"},
				Command: model.Command{Values: []string{"bash"}},
			},
		},
		{
			name: "with-services",
			dev: &model.Dev{
				Name:    "dev",
				Image:   &build.Info{Name: "okteto/dev"},
				Command: model.Command{Values: []string{"bash"}},
				Services: []*model.Dev{{
					Name:    "svc",
					Image:   &build.Info{Name: "okteto/svc"},
					Command: model.Command{Values: []string{"bash"}},
				}, {
					Name:        "svc2",
					Image:       nil,
					Command:     model.Command{Values: []string{"bash"}},
					Environment: []env.Var{{Name: "foo", Value: "bar"}},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			file, err := os.CreateTemp("", "okteto.yml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(file.Name())

			out, err := yaml.Marshal(tt.dev)
			if err != nil {
				t.Fatal(err)
			}

			if _, err = file.Write(out); err != nil {
				t.Fatal("Failed to write to temporary file", err)
			}

			_, err = generateManifestFile(file.Name())
			if err != nil {
				t.Fatal(err)
			}
		})

	}

}
