package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/havoc-io/ssh_config"
	"github.com/okteto/okteto/pkg/config"
)

const (
	sshConfigFile = ".ssh/config"
)

func buildHostname(name string) string {
	return fmt.Sprintf("%s.okteto", name)
}

// AddEntry adds an entry to the user's sshconfig
func AddEntry(name string, port int) error {
	return add(getDefault(), buildHostname(name), port)
}

func add(path string, name string, port int) error {
	cfg, err := getConfig(path)
	if err != nil {
		return err
	}

	_ = removeHost(cfg, name)

	host := ssh_config.NewHost([]string{name}, nil)
	host.Params = []*ssh_config.Param{
		ssh_config.NewParam(ssh_config.HostNameKeyword, []string{"localhost"}, nil),
		ssh_config.NewParam(ssh_config.PortKeyword, []string{strconv.Itoa(port)}, nil),
		ssh_config.NewParam(ssh_config.StrictHostKeyCheckingKeyword, []string{"no"}, nil),
		ssh_config.NewParam(ssh_config.UserKnownHostsFileKeyword, []string{"/dev/null"}, nil),
	}

	cfg.Hosts = append(cfg.Hosts, host)
	return save(cfg, path)
}

// RemoveEntry removes the entry to the user's sshconfig if found
func RemoveEntry(name string) error {
	return remove(getDefault(), buildHostname(name))
}

func remove(path string, name string) error {
	cfg, err := getConfig(path)
	if err != nil {
		return err
	}

	if removeHost(cfg, name) {
		return save(cfg, path)
	}

	return nil
}

func removeHost(cfg *ssh_config.Config, name string) bool {
	ix, ok := findHost(cfg, name)
	if ok {
		cfg.Hosts = append(cfg.Hosts[:ix], cfg.Hosts[ix+1:]...)
		return true
	}

	return false
}

func findHost(cfg *ssh_config.Config, name string) (int, bool) {
	for i, h := range cfg.Hosts {
		for _, hn := range h.Hostnames {
			if hn == name {
				p := h.GetParam(ssh_config.PortKeyword)
				s := h.GetParam(ssh_config.StrictHostKeyCheckingKeyword)
				h := h.GetParam(ssh_config.HostNameKeyword)
				if p != nil && s != nil && h != nil && h.Value() == "localhost" {
					return i, true
				}
			}
		}
	}

	return 0, false
}

func getConfig(path string) (*ssh_config.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ssh_config.Config{
				Hosts: []*ssh_config.Host{},
			}, nil
		}

		return nil, fmt.Errorf("can't open %s: %w", path, err)
	}

	defer f.Close()

	cfg, err := ssh_config.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("fail to decode %s: %w", path, err)
	}

	return cfg, nil
}

func save(cfg *ssh_config.Config, path string) error {
	if err := cfg.WriteToFilepath(path); err != nil {
		if os.IsNotExist(err) {
			_, err = os.Create(path)
			if err != nil {
				return fmt.Errorf("failed to create %s: %w", path, err)
			}

			err = cfg.WriteToFilepath(path)
		}

		if err != nil {
			return fmt.Errorf("fail to save %s: %w", path, err)
		}
	}

	return nil
}

func getDefault() string {
	return filepath.Join(config.GetHomeDir(), ".ssh", "config")
}
