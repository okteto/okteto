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

package okteto

import (
	"io/ioutil"
	"os"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
)

func TestSetKubeConfig(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err.Error())
	}

	defer os.Remove(file.Name())
	c := &Credential{}
	if err := SetKubeConfig(c, file.Name(), "", "123-123-123", "cloud-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeConfig(c, file.Name(), "ns", "123-123-123", "cloud-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeConfig(c, file.Name(), "ns-2", "123-123-123", "cloud-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeConfig(c, file.Name(), "", "123-123-124", "sf-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeConfig(c, file.Name(), "ns-2", "123-123-124", "sf-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	cfg, err := clientcmd.LoadFromFile(file.Name())
	if err != nil {
		t.Fatal(err.Error())
	}

	if len(cfg.Clusters) != 2 {
		t.Errorf("the config file didn't have two clusters: %+v", cfg.Clusters)
	}

	if len(cfg.AuthInfos) != 2 {
		t.Errorf("the config file didn't have two users: %+v", cfg.AuthInfos)
	}

	if len(cfg.Contexts) != 5 {
		t.Errorf("the config file didn't have five contexts: %+v", cfg.Contexts)
	}

	if cfg.CurrentContext != "sf-okteto-com-ns-2" {
		t.Errorf("current context was not sf-okteto-com-ns-2, it was %s", cfg.CurrentContext)
	}

	// add duplicated

	if err := SetKubeConfig(c, file.Name(), "ns-2", "123-123-124", "sf-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeConfig(c, file.Name(), "ns-2", "123-123-123", "cloud-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	cfg, err = clientcmd.LoadFromFile(file.Name())
	if err != nil {
		t.Fatal(err.Error())
	}

	if len(cfg.Contexts) != 5 {
		t.Errorf("the config file didn't have five contexts after adding the same twice: %+v", cfg.Contexts)
	}

}

func TestInDevContainer(t *testing.T) {
	v := os.Getenv("OKTETO_MARKER_PATH")
	os.Setenv("OKTETO_MARKER_PATH", "")
	defer func() {
		os.Setenv("OKTETO_MARKER_PATH", v)
	}()

	in := InDevContainer()
	if in {
		t.Errorf("in dev container when there was no marker env var")
	}

	os.Setenv("OKTETO_MARKER_PATH", "")
	in = InDevContainer()
	if in {
		t.Errorf("in dev container when there was an empty marker env var")
	}

	os.Setenv("OKTETO_MARKER_PATH", "1")
	in = InDevContainer()
	if !in {
		t.Errorf("not in dev container when there was a marker env var")
	}
}
