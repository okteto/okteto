package ssh

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/havoc-io/ssh_config"
)

func Test_add(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(f.Name())

	if err := add(f.Name(), "test.okteto", 8080); err != nil {
		t.Fatal(err)
	}

	if err := add(f.Name(), "test2.okteto", 8081); err != nil {
		t.Fatal(err)
	}

	cfg, err := getConfig(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Hosts) != 2 {
		t.Fatalf("expected 2 hosts got %d", len(cfg.Hosts))
	}

	h := cfg.FindByHostname("test2.okteto")
	if h == nil {
		t.Fatal("couldn't find test2.okteto")
	}

	if err := remove(f.Name(), "test.okteto"); err != nil {
		t.Fatal(err)
	}

	cfg, err = getConfig(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	h = cfg.FindByHostname("test.okteto")
	if h != nil {
		t.Fatal("didn't delete test2.okteto")
	}
}

func Test_removeHost(t *testing.T) {
	tests := []struct {
		name string
		cfg  *ssh_config.Config
		host string
		want bool
	}{
		{
			name: "empty",
			cfg: &ssh_config.Config{
				Hosts: []*ssh_config.Host{},
			},
			want: false,
		},
		{
			name: "single-found",
			cfg: &ssh_config.Config{
				Hosts: []*ssh_config.Host{
					{
						Hostnames: []string{"test.okteto"},
						Params: []*ssh_config.Param{
							ssh_config.NewParam(ssh_config.HostNameKeyword, []string{"localhost"}, nil),
							ssh_config.NewParam(ssh_config.PortKeyword, []string{"8080"}, nil),
							ssh_config.NewParam(ssh_config.StrictHostKeyCheckingKeyword, []string{"no"}, nil),
						},
					},
				},
			},
			host: "test.okteto",
			want: true,
		},
		{
			name: "single-not-found",
			cfg: &ssh_config.Config{
				Hosts: []*ssh_config.Host{
					{
						Hostnames: []string{"test.okteto"},
						Params: []*ssh_config.Param{
							ssh_config.NewParam(ssh_config.HostNameKeyword, []string{"localhost"}, nil),
							ssh_config.NewParam(ssh_config.PortKeyword, []string{"8080"}, nil),
							ssh_config.NewParam(ssh_config.StrictHostKeyCheckingKeyword, []string{"no"}, nil),
						},
					},
				},
			},
			host: "test2.okteto",
			want: false,
		},
		{
			name: "multiple-found",
			cfg: &ssh_config.Config{
				Hosts: []*ssh_config.Host{
					{
						Hostnames: []string{"test.okteto"},
						Params: []*ssh_config.Param{
							ssh_config.NewParam(ssh_config.HostNameKeyword, []string{"localhost"}, nil),
							ssh_config.NewParam(ssh_config.PortKeyword, []string{"8080"}, nil),
							ssh_config.NewParam(ssh_config.StrictHostKeyCheckingKeyword, []string{"no"}, nil),
						},
					},
					{
						Hostnames: []string{"test2.okteto"},
						Params: []*ssh_config.Param{
							ssh_config.NewParam(ssh_config.HostNameKeyword, []string{"localhost"}, nil),
							ssh_config.NewParam(ssh_config.PortKeyword, []string{"8080"}, nil),
							ssh_config.NewParam(ssh_config.StrictHostKeyCheckingKeyword, []string{"no"}, nil),
						},
					},
					{
						Hostnames: []string{"test3.okteto"},
						Params: []*ssh_config.Param{
							ssh_config.NewParam(ssh_config.HostNameKeyword, []string{"localhost"}, nil),
							ssh_config.NewParam(ssh_config.PortKeyword, []string{"8080"}, nil),
							ssh_config.NewParam(ssh_config.StrictHostKeyCheckingKeyword, []string{"no"}, nil),
						},
					},
				},
			},
			host: "test2.okteto",
			want: true,
		},
		{
			name: "multiple-not-found",
			cfg: &ssh_config.Config{
				Hosts: []*ssh_config.Host{
					{
						Hostnames: []string{"test.okteto"},
						Params: []*ssh_config.Param{
							ssh_config.NewParam(ssh_config.HostNameKeyword, []string{"localhost"}, nil),
							ssh_config.NewParam(ssh_config.PortKeyword, []string{"8080"}, nil),
							ssh_config.NewParam(ssh_config.StrictHostKeyCheckingKeyword, []string{"no"}, nil),
						},
					},
					{
						Hostnames: []string{"test2.okteto"},
						Params: []*ssh_config.Param{
							ssh_config.NewParam(ssh_config.HostNameKeyword, []string{"localhost"}, nil),
							ssh_config.NewParam(ssh_config.PortKeyword, []string{"8080"}, nil),
							ssh_config.NewParam(ssh_config.StrictHostKeyCheckingKeyword, []string{"no"}, nil),
						},
					},
					{
						Hostnames: []string{"test3.okteto"},
						Params: []*ssh_config.Param{
							ssh_config.NewParam(ssh_config.HostNameKeyword, []string{"localhost"}, nil),
							ssh_config.NewParam(ssh_config.PortKeyword, []string{"8080"}, nil),
							ssh_config.NewParam(ssh_config.StrictHostKeyCheckingKeyword, []string{"no"}, nil),
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
