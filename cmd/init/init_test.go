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

package init

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/okteto/okteto/cmd/utils"
)

func TestRun(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dir)

	p := filepath.Join(dir, fmt.Sprintf("okteto-%s", uuid.New().String()))
	if err := Run("", p, "golang", dir, false); err != nil {
		t.Fatal(err)
	}

	d, err := utils.LoadDev(p)
	if err != nil {
		t.Fatal(err)
	}

	if d.Image != "okteto/golang:1" {
		t.Errorf("got %s, expected %s", d.Image, "okteto/golang:1")
	}

	if err := Run("", p, "java", dir, false); err == nil {
		t.Fatalf("manifest was overwritten: %s", err)
	}

	if err := Run("", p, "ruby", dir, true); err != nil {
		t.Fatalf("manifest wasn't overwritten: %s", err)
	}

	d, err = utils.LoadDev(p)
	if err != nil {
		t.Fatal(err)
	}

	if d.Image != "okteto/ruby:2" {
		t.Errorf("got %s, expected %s", d.Image, "okteto/ruby:2")
	}
}
