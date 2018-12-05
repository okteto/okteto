package syncthing

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"
)

var (
	configTemplate = template.Must(template.New("syncthingConfig").Parse(configXML))
)

const (
	certFile   = "cert.pem"
	keyFile    = "key.pem"
	configFile = "config.xml"
	portFile   = ".port"
	logFile    = "syncthing.log"

	// DefaultRemoteDeviceID remote syncthing ID
	DefaultRemoteDeviceID = "ATOPHFJ-VPVLDFY-QVZDCF2-OQQ7IOW-OG4DIXF-OA7RWU3-ZYA4S22-SI4XVAU"

	// DefaultFileWatcherDelay how much to wait before starting a sync after a file change
	DefaultFileWatcherDelay = 5
)

// Syncthing represents the local syncthing process.
type Syncthing struct {
	cmd              *exec.Cmd
	binPath          string
	Home             string
	Name             string
	Namespace        string
	LocalPath        string
	RemoteAddress    string
	RemoteDeviceID   string
	APIKey           string
	FileWatcherDelay int
	GUIAddress       string
	ListenAddress    string
}

// NewSyncthing constructs a new Syncthing.
func NewSyncthing(name, namespace, localPath string) (*Syncthing, error) {

	remotePort, err := getAvailablePort()
	if err != nil {
		return nil, err
	}

	guiPort, err := getAvailablePort()
	if err != nil {
		return nil, err
	}

	listenPort, err := getAvailablePort()
	if err != nil {
		return nil, err
	}

	s := &Syncthing{
		APIKey:           "cnd",
		binPath:          "syncthing",
		Name:             name,
		Namespace:        namespace,
		Home:             path.Join(os.Getenv("HOME"), ".cnd", namespace, name),
		LocalPath:        localPath,
		RemoteAddress:    fmt.Sprintf("tcp://localhost:%d", remotePort),
		RemoteDeviceID:   DefaultRemoteDeviceID,
		FileWatcherDelay: DefaultFileWatcherDelay,
		GUIAddress:       fmt.Sprintf("127.0.0.1:%d", guiPort),
		ListenAddress:    fmt.Sprintf("0.0.0.0:%d", listenPort),
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
		"-logfile", path.Join(s.Home, logFile),
	}

	s.cmd = exec.Command(s.binPath, cmdArgs...) //nolint: gas, gosec
	s.cmd.Env = append(os.Environ(), "STNOUPGRADE=1")

	if err := s.cmd.Start(); err != nil {
		return err
	}

	if s.cmd.Process == nil {
		return nil
	}

	if err := ioutil.WriteFile(
		pidPath,
		[]byte(strconv.Itoa(s.cmd.Process.Pid)),
		0600); err != nil {
		return err
	}

	log.Infof("Syncthing running on http://%s and tcp://%s", s.GUIAddress, s.ListenAddress)
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
