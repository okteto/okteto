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

package doctor

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/model"
)

func Test_generateManifestFile(t *testing.T) {
	var tests = []struct {
		name      string
		dev       *model.Dev
		expectErr bool
	}{
		{
			name:      "empty",
			dev:       nil,
			expectErr: true,
		},
		{
			name: "basic",
			dev: &model.Dev{
				Name:    "dev",
				Image:   &model.BuildInfo{Name: "okteto/dev"},
				Command: model.Command{Values: []string{"bash"}},
			},
			expectErr: false,
		},
		{
			name: "with-services",
			dev: &model.Dev{
				Name:    "dev",
				Image:   &model.BuildInfo{Name: "okteto/dev"},
				Command: model.Command{Values: []string{"bash"}},
				Services: []*model.Dev{{
					Name:    "svc",
					Image:   &model.BuildInfo{Name: "okteto/svc"},
					Command: model.Command{Values: []string{"bash"}},
				}},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			file, err := ioutil.TempFile("", "okteto.yml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(file.Name())

			_, err = generateManifestFile(context.TODO(), file.Name())
			if err != nil {
				if tt.expectErr {
					return
				}

				t.Fatal(err)
			}
		})

	}

}
