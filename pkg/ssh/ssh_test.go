package ssh

import (
	"io/ioutil"
	"os"
	"testing"
	"path/filepath"
)

func Test_add(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dir)
	sshConfig := filepath.Join(dir, "config")

	if err := add(sshConfig, "test.okteto", 8080); err != nil {
		t.Fatal(err)
	}

	if err := add(sshConfig, "test2.okteto", 8081); err != nil {
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
							newParam(hostNameKeyword, []string{"localhost"}, nil),
							newParam(portKeyword, []string{"8080"}, nil),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}, nil),
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
							newParam(hostNameKeyword, []string{"localhost"}, nil),
							newParam(portKeyword, []string{"8080"}, nil),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}, nil),
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
							newParam(hostNameKeyword, []string{"localhost"}, nil),
							newParam(portKeyword, []string{"8080"}, nil),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}, nil),
						},
					},
					{
						hostnames: []string{"test2.okteto"},
						params: []*param{
							newParam(hostNameKeyword, []string{"localhost"}, nil),
							newParam(portKeyword, []string{"8080"}, nil),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}, nil),
						},
					},
					{
						hostnames: []string{"test3.okteto"},
						params: []*param{
							newParam(hostNameKeyword, []string{"localhost"}, nil),
							newParam(portKeyword, []string{"8080"}, nil),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}, nil),
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
							newParam(hostNameKeyword, []string{"localhost"}, nil),
							newParam(portKeyword, []string{"8080"}, nil),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}, nil),
						},
					},
					{
						hostnames: []string{"test2.okteto"},
						params: []*param{
							newParam(hostNameKeyword, []string{"localhost"}, nil),
							newParam(portKeyword, []string{"8080"}, nil),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}, nil),
						},
					},
					{
						hostnames: []string{"test3.okteto"},
						params: []*param{
							newParam(hostNameKeyword, []string{"localhost"}, nil),
							newParam(portKeyword, []string{"8080"}, nil),
							newParam(strictHostKeyCheckingKeyword, []string{"no"}, nil),
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
