package syncthing

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
)

var (
	configTemplate = template.Must(template.New("syncthingConfig").Parse(configXML))
)

const (
	certFile   = "cert.pem"
	keyFile    = "key.pem"
	configFile = "config.xml"

	// DefaultRemoteDeviceID remote syncthing ID
	DefaultRemoteDeviceID = "ATOPHFJ-VPVLDFY-QVZDCF2-OQQ7IOW-OG4DIXF-OA7RWU3-ZYA4S22-SI4XVAU"
)

// Syncthing represents the local syncthing process.
type Syncthing struct {
	cmd            *exec.Cmd
	Home           string
	Name           string
	LocalPath      string
	RemoteAddress  string
	RemoteDeviceID string
	APIKey         string
}

// NewSyncthing constructs a new Syncthing.
func NewSyncthing(name, namespace, localPath, remoteDeviceID, remoteAddress string) *Syncthing {
	return &Syncthing{
		APIKey:         "cnd",
		Name:           name,
		Home:           path.Join(os.Getenv("HOME"), ".oksync", namespace, name),
		LocalPath:      localPath,
		RemoteAddress:  remoteAddress,
		RemoteDeviceID: remoteDeviceID,
	}
}

// Normally, syscall.Kill would be good enough. Unfortunately, that's not
// supported in windows. While this isn't tested on windows it at least gets
// past the compiler.
func (s *Syncthing) cleanupDaemon(pidPath string) error {
	// Deal with Windows conditions by bailing
	if runtime.GOOS == "windows" {
		return nil
	}

	if _, err := os.Stat(pidPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	content, err := ioutil.ReadFile(pidPath) // nolint: gosec
	if err != nil {
		return err
	}

	pid, pidErr := strconv.Atoi(string(content))
	if pidErr != nil {
		return pidErr
	}

	proc := os.Process{Pid: pid}

	if err := proc.Signal(os.Interrupt); err != nil {
		if strings.Contains(err.Error(), "process already finished") {
			return nil
		}

		return err
	}

	defer proc.Wait() // nolint: errcheck

	return nil
}

func (s *Syncthing) initConfig() error {
	os.MkdirAll(s.Home, 0700)

	buf := new(bytes.Buffer)
	if err := configTemplate.Execute(buf, s); err != nil {
		return err
	}

	if err := ioutil.WriteFile(path.Join(s.Home, configFile), buf.Bytes(), 0700); err != nil {
		return err
	}

	if err := ioutil.WriteFile(path.Join(s.Home, certFile), cert, 0700); err != nil {
		return err
	}

	if err := ioutil.WriteFile(path.Join(s.Home, keyFile), key, 0700); err != nil {
		return err
	}

	return nil
}

// Run starts up a local syncthing process to serve files from.
func (s *Syncthing) Run() error {
	if err := s.initConfig(); err != nil {
		return err
	}

	pidPath := filepath.Join(s.Home, "syncthing.pid")

	if err := s.cleanupDaemon(pidPath); err != nil {
		return err
	}

	// TODO calculate the path or include it in the release
	path := "/usr/local/bin/syncthing"

	cmdArgs := []string{
		"-home", s.Home,
		"-no-browser",
		"-verbose",
	}

	s.cmd = exec.Command(path, cmdArgs...) //nolint: gas, gosec

	if err := s.cmd.Start(); err != nil {
		return err
	}

	//Because child process signal handling is completely broken, just save the
	//pid and try to kill it every start.
	if s.cmd.Process == nil {
		return nil
	}

	if err := ioutil.WriteFile(
		pidPath,
		[]byte(strconv.Itoa(s.cmd.Process.Pid)),
		0600); err != nil {
		return err
	}

	return nil
}

// Stop halts the background process and cleans up.
func (s *Syncthing) Stop() error {
	pidPath := filepath.Join(s.Home, "syncthing.pid")

	if err := s.cleanupDaemon(pidPath); err != nil {
		return err
	}

	return nil
}
