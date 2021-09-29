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

package init

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/okteto"
	"gopkg.in/yaml.v2"
)

func TestMain(m *testing.M) {
	okteto.CurrentStore = &okteto.OktetoContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Name:      "test",
				Namespace: "namespace",
				UserID:    "user-id",
			},
		},
	}
	os.Exit(m.Run())
}

func TestRun(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dir)

	p := filepath.Join(dir, fmt.Sprintf("okteto-%s", uuid.New().String()))
	if err := Run(p, "golang", dir, false); err != nil {
		t.Fatal(err)
	}

	stignorePath := filepath.Join(dir, ".stignore")
	if _, err := os.Stat(stignorePath); os.IsNotExist(err) {
		t.Fatal(err)
	}

	fmt.Println("1")
	dev, err := utils.LoadDev(p, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if dev.Image.Name != "okteto/golang:1" {
		t.Errorf("got %s, expected %s", dev.Image, "okteto/golang:1")
	}

	fmt.Println("2")
	if err := Run(p, "ruby", dir, true); err != nil {
		t.Fatalf("manifest wasn't overwritten: %s", err)
	}

	fmt.Println("3")
	dev, err = utils.LoadDev(p, "", "")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("4")
	if dev.Image.Name != "okteto/ruby:2" {
		t.Errorf("got %s, expected %s", dev.Image, "okteto/ruby:2")
	}
}

func TestRunJustCreateNecessaryFields(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dir)

	p := filepath.Join(dir, fmt.Sprintf("okteto-%s", uuid.New().String()))
	if err := Run(p, "golang", dir, false); err != nil {
		t.Fatal(err)
	}

	file, _ := ioutil.ReadFile(p)
	var result map[string]interface{}
	yaml.Unmarshal([]byte(file), &result)

	optionalFields := [...]string{"annotations", "autocreate", "container", "context", "environment",
		"externalVolumes", "healthchecks", "interface", "imagePullPolicy", "labels", "namespace",
		"push", "resources", "remote", "reverse", "secrets", "services", "subpath",
		"tolerations", "workdir"}
	for _, field := range optionalFields {
		if _, ok := result[field]; ok {
			t.Fatal(fmt.Errorf("field '%s' in manifest after running `okteto init` and its not necessary", field))
		}
	}

}
