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
	if err := SetKubeContext(c, file.Name(), "", "123-123-123", "cloud-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeContext(c, file.Name(), "ns", "123-123-123", "cloud-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeContext(c, file.Name(), "ns-2", "123-123-123", "cloud-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeContext(c, file.Name(), "", "123-123-124", "sf-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeContext(c, file.Name(), "ns-2", "123-123-124", "sf-okteto-com"); err != nil {
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

	if len(cfg.Contexts) != 2 {
		t.Errorf("the config file didn't have five contexts: %+v", cfg.Contexts)
	}

	if cfg.CurrentContext != "sf-okteto-com" {
		t.Errorf("current context was not sf-okteto-com, it was %s", cfg.CurrentContext)
	}

	// add duplicated

	if err := SetKubeContext(c, file.Name(), "ns-2", "123-123-124", "sf-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	if err := SetKubeContext(c, file.Name(), "ns-2", "123-123-123", "cloud-okteto-com"); err != nil {
		t.Fatal(err.Error())
	}

	cfg, err = clientcmd.LoadFromFile(file.Name())
	if err != nil {
		t.Fatal(err.Error())
	}

	if len(cfg.Contexts) != 2 {
		t.Fatalf("the config file didn't have five contexts after adding the same twice: %+v", cfg.Contexts)
	}
}

func TestInDevContainer(t *testing.T) {
	v := os.Getenv("OKTETO_NAME")
	os.Setenv("OKTETO_NAME", "")
	defer func() {
		os.Setenv("OKTETO_NAME", v)
	}()

	in := InDevContainer()
	if in {
		t.Errorf("in dev container when there was no marker env var")
	}

	os.Setenv("OKTETO_NAME", "")
	in = InDevContainer()
	if in {
		t.Errorf("in dev container when there was an empty marker env var")
	}

	os.Setenv("OKTETO_NAME", "1")
	in = InDevContainer()
	if !in {
		t.Errorf("not in dev container when there was a marker env var")
	}
}

func Test_parseOktetoURL(t *testing.T) {
	tests := []struct {
		name    string
		u       string
		want    string
		wantErr bool
	}{
		{
			name: "basic",
			u:    "https://cloud.okteto.com",
			want: "https://cloud.okteto.com/graphql",
		},
		{
			name: "no-schema",
			u:    "cloud.okteto.com",
			want: "https://cloud.okteto.com/graphql",
		},
		{
			name:    "empty",
			u:       "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOktetoURL(tt.u)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOktetoURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseOktetoURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
