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

package manifest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestMain(m *testing.M) {
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

func TestRun(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	p := filepath.Join(dir, fmt.Sprintf("okteto-%s", uuid.New().String()))

	mc := &Command{}
	opts := &InitOpts{
		DevPath:  p,
		Language: "golang",
		Workdir:  dir,
	}

	require.NoError(t, mc.RunInitV1(ctx, opts))

	stignorePath := filepath.Join(dir, ".stignore")
	_, err := os.Stat(stignorePath)
	require.NoError(t, err)

	manifest, err := utils.DeprecatedLoadManifest(p)
	require.NoError(t, err)

	dev, err := utils.GetDevFromManifest(manifest, "")
	require.NoError(t, err)

	if dev.Image.Name != "okteto/golang:1" {
		t.Errorf("got %s, expected %s", dev.Image.Name, "okteto/golang:1")
	}

	opts = &InitOpts{
		DevPath:   p,
		Language:  "ruby",
		Workdir:   dir,
		Overwrite: true,
	}

	if err := mc.RunInitV1(ctx, opts); err != nil {
		t.Fatalf("manifest wasn't overwritten: %s", err)
	}

	manifest, err = utils.DeprecatedLoadManifest(p)
	require.NoError(t, err)

	dev, err = utils.GetDevFromManifest(manifest, "")
	require.NoError(t, err)

	if dev.Image.Name != "okteto/ruby:2" {
		t.Errorf("got %s, expected %s", dev.Image.Name, "okteto/ruby:2")
	}
}

func TestRunJustCreateNecessaryFields(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	mc := &Command{}
	p := filepath.Join(dir, fmt.Sprintf("okteto-%s", uuid.New().String()))
	opts := &InitOpts{
		DevPath:  p,
		Language: "golang",
		Workdir:  dir,
	}
	require.NoError(t, mc.RunInitV1(ctx, opts))

	file, err := os.ReadFile(p)
	assert.NoError(t, err)
	var result map[string]interface{}
	require.NoError(t, yaml.Unmarshal(file, &result))

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
