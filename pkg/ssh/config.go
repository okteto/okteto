// based on https://github.com/havoc-io/ssh_config

package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
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
	hostKeyword                  = "Host"
	hostNameKeyword              = "HostName"
	portKeyword                  = "Port"
	strictHostKeyCheckingKeyword = "StrictHostKeyChecking"
	userKnownHostsFileKeyword    = "UserKnownHostsFile"
)

func newHost(hostnames []string, comments []string) *host {
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

func newParam(keyword string, args []string, comments []string) *param {
	return &param{
		comments: comments,
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

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	config := &sshConfig{
		source: data,
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {

		line := strings.TrimSpace(sc.Text())
		if len(line) == 0 {
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
	for _, param := range config.globals {
		fmt.Fprint(w, param.String())
	}

	if len(config.globals) > 0 {
		fmt.Fprintln(w)
	}

	for _, host := range config.hosts {
		//fmt.Fprintln(w)
		fmt.Fprint(w, host.String())
	}

	return nil
}

func (config *sshConfig) writeToFilepath(filePath string) error {

	// create a tmp file in the same path with the same mode
	tmpFilePath := filePath + "." + strconv.FormatInt(time.Now().UnixNano(), 10)

	stat, err := os.Stat(filePath)
	var mode os.FileMode
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		// default for ssh_config
		mode = 0600
	} else {
		mode = stat.Mode()
	}

	file, err := os.OpenFile(tmpFilePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL|os.O_SYNC, mode)
	if err != nil {
		return err
	}

	if err := config.writeTo(file); err != nil {
		file.Close()
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpFilePath, filePath); err != nil {
		return err
	}

	return nil

}

func (config *sshConfig) getParam(keyword string) *param {
	for _, p := range config.globals {
		if p.keyword == keyword {
			return p
		}
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
