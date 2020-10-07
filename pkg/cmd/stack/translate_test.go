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

package stack

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/model"
)

const (
	env = `A=1
# comment


B=$B

C=3`
)

func Test_translateEnvVars(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", ".env")
	if err != nil {
		t.Fatalf("failed to create dynamic env file: %s", err.Error())
	}
	if err := ioutil.WriteFile(tmpFile.Name(), []byte(env), 0600); err != nil {
		t.Fatalf("failed to write env file: %s", err.Error())
	}
	defer os.RemoveAll(tmpFile.Name())

	os.Setenv("IMAGE", "image")
	os.Setenv("B", "2")
	os.Setenv("ENV_PATH", tmpFile.Name())
	stack := &model.Stack{
		Name: "name",
		Services: map[string]model.Service{
			"1": {
				Image:    "${IMAGE}",
				EnvFiles: []string{"${ENV_PATH}"},
				Environment: []model.EnvVar{
					{
						Name:  "C",
						Value: "original",
					},
				},
			},
		},
	}
	translateEnvVars(stack)
	if stack.Services["1"].Image != "image" {
		t.Errorf("Wrong image: %s", stack.Services["1"].Image)
	}
	if len(stack.Services["1"].Environment) != 3 {
		t.Errorf("Wrong envirironment: %v", stack.Services["1"].Environment)
	}
	for _, e := range stack.Services["1"].Environment {
		if e.Name == "A" && e.Value != "1" {
			t.Errorf("Wrong envirironment variable A: %s", e.Value)
		}
		if e.Name == "B" && e.Value != "2" {
			t.Errorf("Wrong envirironment variable B: %s", e.Value)
		}
		if e.Name == "C" && e.Value != "original" {
			t.Errorf("Wrong envirironment variable C: %s", e.Value)
		}
	}
}
