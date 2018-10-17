package syncthing

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
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
	portFile   = ".port"

	// DefaultRemoteDeviceID remote syncthing ID
	DefaultRemoteDeviceID = "ATOPHFJ-VPVLDFY-QVZDCF2-OQQ7IOW-OG4DIXF-OA7RWU3-ZYA4S22-SI4XVAU"
)

// Syncthing represents the local syncthing process.
type Syncthing struct {
	cmd            *exec.Cmd
	BinPath        string
	Home           string
	Name           string
	LocalPath      string
	RemoteAddress  string
	RemoteDeviceID string
	APIKey         string
	GUIAddress     string
}

// NewSyncthing constructs a new Syncthing.
func NewSyncthing(name, namespace, localPath string) (*Syncthing, error) {

	port, err := getAvailablePort()
	if err != nil {
		return nil, err
	}

	guiPort, err := getAvailablePort()
	if err != nil {
		return nil, err
	}

	s := &Syncthing{
		APIKey:         "cnd",
		BinPath:        "syncthing",
		Name:           name,
		Home:           path.Join(os.Getenv("HOME"), ".cnd", namespace, name),
		LocalPath:      localPath,
		RemoteAddress:  fmt.Sprintf("tcp://localhost:%d", port),
		RemoteDeviceID: DefaultRemoteDeviceID,
		GUIAddress:     fmt.Sprintf("http://127.0.0.1:%d", guiPort),
	}

	if err := s.initConfig(); err != nil {
		return nil, err
	}

	return s, nil
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

func getAvailablePort() (int, error) {
	address, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	listener, err := net.ListenTCP("tcp", address)
	if err != nil {
		return 0, err
	}

	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil

}

// Run starts up a local syncthing process to serve files from.
func (s *Syncthing) Run() error {
	pidPath := filepath.Join(s.Home, "syncthing.pid")

	if err := s.cleanupDaemon(pidPath); err != nil {
		return err
	}

	cmdArgs := []string{
		"-home", s.Home,
		"-no-browser",
		"-verbose",
		"-gui-address", s.GUIAddress,
	}

	s.cmd = exec.Command(s.BinPath, cmdArgs...) //nolint: gas, gosec

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

	log.Printf("syncthing running on %s", s.GUIAddress)
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
