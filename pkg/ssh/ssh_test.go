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

package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
)

func Test_addOnEmpty(t *testing.T) {
	dir := t.TempDir()

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}

	sshConfig := filepath.Join(dir, "config")

	if err := add(sshConfig, "test.okteto", model.Localhost, 8080); err != nil {
		t.Fatal(err)
	}

	cfg, err := getConfig(sshConfig)
	if err != nil {
		t.Fatal(err)
	}

	h := cfg.getHost("test.okteto")
	if h == nil {
		t.Fatal("couldn't find test.okteto")
	}

	if err := remove(sshConfig, "test.okteto"); err != nil {
		t.Fatal(err)
	}
}
func Test_add(t *testing.T) {
	dir := t.TempDir()
	sshConfig := filepath.Join(dir, "config")

	if err := add(sshConfig, "test.okteto", model.Localhost, 8080); err != nil {
		t.Fatal(err)
	}

	if err := add(sshConfig, "test2.okteto", model.Localhost, 8081); err != nil {
		t.Fatal(err)
	}

	cfg, err := getConfig(sshConfig)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.hosts) != 2 {
		t.Fatalf("expected 2 hosts got %d", len(cfg.hosts))
	}

	h := cfg.getHost("test2.okteto")
	if h == nil {
		t.Fatal("couldn't find test2.okteto")
	}

	if err := remove(sshConfig, "test.okteto"); err != nil {
		t.Fatal(err)
	}

	cfg, err = getConfig(sshConfig)
	if err != nil {
		t.Fatal(err)
	}

	h = cfg.getHost("test.okteto")
	if h != nil {
		t.Fatal("didn't delete test2.okteto")
	}
}

func Test_removeHost(t *testing.T) {
	tests := []struct {
		name string
		cfg  *sshConfig
		host string
		want bool
	}{
		{
			name: "empty",
			cfg: &sshConfig{
				hosts: []*host{},
			},
			want: false,
		},
		{
			name: "single-found",
			cfg: &sshConfig{
				hosts: []*host{
					{
						hostnames: []string{"test.okteto"},
						params: []*param{
							newParam(hostNameKeyword, []string{model.Localhost}),
							newParam(portKeyword, []string{"8080"}),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}),
						},
					},
				},
			},
			host: "test.okteto",
			want: true,
		},
		{
			name: "single-not-found",
			cfg: &sshConfig{
				hosts: []*host{
					{
						hostnames: []string{"test.okteto"},
						params: []*param{
							newParam(hostNameKeyword, []string{model.Localhost}),
							newParam(portKeyword, []string{"8080"}),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}),
						},
					},
				},
			},
			host: "test2.okteto",
			want: false,
		},
		{
			name: "multiple-found",
			cfg: &sshConfig{
				hosts: []*host{
					{
						hostnames: []string{"test.okteto"},
						params: []*param{
							newParam(hostNameKeyword, []string{model.Localhost}),
							newParam(portKeyword, []string{"8080"}),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}),
						},
					},
					{
						hostnames: []string{"test2.okteto"},
						params: []*param{
							newParam(hostNameKeyword, []string{model.Localhost}),
							newParam(portKeyword, []string{"8080"}),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}),
						},
					},
					{
						hostnames: []string{"test3.okteto"},
						params: []*param{
							newParam(hostNameKeyword, []string{model.Localhost}),
							newParam(portKeyword, []string{"8080"}),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}),
						},
					},
				},
			},
			host: "test2.okteto",
			want: true,
		},
		{
			name: "multiple-not-found",
			cfg: &sshConfig{
				hosts: []*host{
					{
						hostnames: []string{"test.okteto"},
						params: []*param{
							newParam(hostNameKeyword, []string{model.Localhost}),
							newParam(portKeyword, []string{"8080"}),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}),
						},
					},
					{
						hostnames: []string{"test2.okteto"},
						params: []*param{
							newParam(hostNameKeyword, []string{model.Localhost}),
							newParam(portKeyword, []string{"8080"}),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}),
						},
					},
					{
						hostnames: []string{"test3.okteto"},
						params: []*param{
							newParam(hostNameKeyword, []string{model.Localhost}),
							newParam(portKeyword, []string{"8080"}),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}),
						},
					},
				},
			},
			host: "test4.okteto",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeHost(tt.cfg, tt.host); got != tt.want {
				t.Errorf("removeHost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPort(t *testing.T) {
	dir := t.TempDir()

	t.Setenv(constants.OktetoHomeEnvVar, dir)

	defer os.Unsetenv(constants.OktetoHomeEnvVar)

	if _, err := GetPort(t.Name()); err == nil {
		t.Fatal("expected error on non existing host")
	}

	if err := AddEntry(t.Name(), "localhost", 123456); err != nil {
		t.Fatal(err)
	}

	p, err := GetPort(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	if p != 123456 {
		t.Errorf("got %d, expected %d", p, 123456)
	}

}

func Test_getSSHConfigPath(t *testing.T) {
	ssh := getSSHConfigPath()
	parts := strings.Split(ssh, string(os.PathSeparator))
	found := false
	for _, p := range parts {
		if p == ".ssh" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("didn't found the correct .ssh folder: %s", parts)
	}
}
