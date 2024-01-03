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

// based on https://github.com/havoc-io/ssh_config
package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

type (
	sshConfig struct {
		source  []byte
		globals []*param
		hosts   []*host
	}
	host struct {
		comments  []string
		hostnames []string
		params    []*param
	}
	param struct {
		comments []string
		keyword  string
		args     []string
	}
)

const (
	forwardAgentKeyword           = "ForwardAgent"
	pubkeyAcceptedKeyTypesKeyword = "PubkeyAcceptedKeyTypes"
	hostKeyword                   = "Host"
	hostNameKeyword               = "HostName"
	portKeyword                   = "Port"
	strictHostKeyCheckingKeyword  = "StrictHostKeyChecking"
	hostKeyAlgorithms             = "HostKeyAlgorithms"
	userKnownHostsFileKeyword     = "UserKnownHostsFile"
	identityFile                  = "IdentityFile"
	identitiesOnly                = "IdentitiesOnly"
)

func newHost(hostnames, comments []string) *host {
	return &host{
		comments:  comments,
		hostnames: hostnames,
	}
}

func (h *host) String() string {

	buf := &bytes.Buffer{}

	if len(h.comments) > 0 {
		for _, comment := range h.comments {
			if !strings.HasPrefix(comment, "#") {
				comment = "# " + comment
			}
			fmt.Fprintln(buf, comment)
		}
	}

	fmt.Fprintf(buf, "%s %s\n", hostKeyword, strings.Join(h.hostnames, " "))
	for _, param := range h.params {
		fmt.Fprint(buf, "  ", param.String())
	}

	return buf.String()

}

func newParam(keyword string, args []string) *param {
	return &param{
		comments: nil,
		keyword:  keyword,
		args:     args,
	}
}

func (p *param) String() string {

	buf := &bytes.Buffer{}

	if len(p.comments) > 0 {
		fmt.Fprintln(buf)
		for _, comment := range p.comments {
			if !strings.HasPrefix(comment, "#") {
				comment = "# " + comment
			}
			fmt.Fprintln(buf, comment)
		}
	}

	fmt.Fprintf(buf, "%s %s\n", p.keyword, strings.Join(p.args, " "))

	return buf.String()

}

func (p *param) value() string {
	if len(p.args) > 0 {
		return p.args[0]
	}
	return ""
}

func parse(r io.Reader) (*sshConfig, error) {

	// dat state
	var (
		global = true

		p = &param{}
		h *host
	)

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	config := &sshConfig{
		source: data,
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {

		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		if line[0] == '#' {
			p.comments = append(p.comments, line)
			continue
		}

		psc := bufio.NewScanner(strings.NewReader(line))
		psc.Split(bufio.ScanWords)
		if !psc.Scan() {
			continue
		}

		p.keyword = psc.Text()

		for psc.Scan() {
			p.args = append(p.args, psc.Text())
		}

		if p.keyword == hostKeyword {
			global = false
			if h != nil {
				config.hosts = append(config.hosts, h)
			}
			h = &host{
				comments:  p.comments,
				hostnames: p.args,
			}
			p = &param{}
			continue
		} else if global {
			config.globals = append(config.globals, p)
			p = &param{}
			continue
		}

		h.params = append(h.params, p)
		p = &param{}

	}

	if global {
		config.globals = append(config.globals, p)
	} else if h != nil {
		config.hosts = append(config.hosts, h)
	}

	return config, nil

}

func (config *sshConfig) writeTo(w io.Writer) error {
	buf := bytes.NewBufferString("")
	for _, param := range config.globals {
		if _, err := fmt.Fprint(buf, param.String()); err != nil {
			return err
		}
	}

	if len(config.globals) > 0 {
		if _, err := fmt.Fprintln(buf); err != nil {
			return err
		}
	}

	for _, host := range config.hosts {
		if _, err := fmt.Fprint(buf, host.String()); err != nil {
			return err
		}
	}

	_, err := fmt.Fprint(w, buf.String())
	return err
}

func (config *sshConfig) writeToFilepath(p string) error {
	sshDir := filepath.Dir(p)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		oktetoLog.Infof("failed to create SSH directory %s: %s", sshDir, err)
	}

	stat, err := os.Stat(p)
	var mode os.FileMode
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to get info on %s: %w", p, err)
		}

		// default for ssh_config
		mode = 0600
	} else {
		mode = stat.Mode()
	}

	dir := filepath.Dir(p)
	temp, err := os.CreateTemp(dir, "")
	if err != nil {
		return fmt.Errorf("failed to create temporary config file: %w", err)
	}

	defer os.Remove(temp.Name())

	if err := config.writeTo(temp); err != nil {
		return err
	}

	if err := temp.Close(); err != nil {
		return err
	}

	if err := os.Chmod(temp.Name(), mode); err != nil {
		return fmt.Errorf("failed to set permissions to %s: %w", temp.Name(), err)
	}

	if _, err := getConfig(temp.Name()); err != nil {
		return fmt.Errorf("new config is not valid: %w", err)
	}

	if err := os.Rename(temp.Name(), p); err != nil {
		return fmt.Errorf("failed to move %s to %s: %w", temp.Name(), p, err)
	}

	return nil

}

func (config *sshConfig) getHost(hostname string) *host {
	for _, host := range config.hosts {
		for _, hn := range host.hostnames {
			if hn == hostname {
				return host
			}
		}
	}
	return nil
}

func (h *host) getParam(keyword string) *param {
	for _, p := range h.params {
		if p.keyword == keyword {
			return p
		}
	}
	return nil
}
