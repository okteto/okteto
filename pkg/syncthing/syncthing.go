package syncthing

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/cloudnativedevelopment/cnd/pkg/config"
	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/model"

	ps "github.com/mitchellh/go-ps"
)

var (
	configTemplate = template.Must(template.New("syncthingConfig").Parse(configXML))
)

const (
	binaryName       = "syncthing"
	certFile         = "cert.pem"
	keyFile          = "key.pem"
	configFile       = "config.xml"
	portFile         = ".port"
	logFile          = "syncthing.log"
	syncthingPidFile = "syncthing.pid"

	// DefaultRemoteDeviceID remote syncthing ID
	DefaultRemoteDeviceID = "ATOPHFJ-VPVLDFY-QVZDCF2-OQQ7IOW-OG4DIXF-OA7RWU3-ZYA4S22-SI4XVAU"

	// DefaultFileWatcherDelay how much to wait before starting a sync after a file change
	DefaultFileWatcherDelay = 5
)

// Syncthing represents the local syncthing process.
type Syncthing struct {
	cmd              *exec.Cmd
	binPath          string
	home             string
	Name             string
	DevList          []*model.Dev
	Namespace        string
	RemoteAddress    string
	RemoteDeviceID   string
	APIKey           string
	FileWatcherDelay int
	GUIAddress       string
	ListenAddress    string
	Client           *http.Client
}

// NewSyncthing constructs a new Syncthing.
func NewSyncthing(namespace, deployment string, devList []*model.Dev) (*Syncthing, error) {

	fullPath := GetInstallPath()
	if !IsInstalled() {
		return nil, fmt.Errorf("cannot find syncthing. Make sure syncthing is installed in %s", fullPath)
	}

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
		binPath:          fullPath,
		home:             path.Join(config.GetCNDHome(), namespace, deployment),
		Name:             deployment,
		DevList:          devList,
		Namespace:        namespace,
		RemoteAddress:    fmt.Sprintf("tcp://localhost:%d", remotePort),
		RemoteDeviceID:   DefaultRemoteDeviceID,
		FileWatcherDelay: DefaultFileWatcherDelay,
		GUIAddress:       fmt.Sprintf("127.0.0.1:%d", guiPort),
		ListenAddress:    fmt.Sprintf("0.0.0.0:%d", listenPort),
		Client:           NewAPIClient(),
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

	pid, err := getPID(pidPath)
	if os.IsNotExist(err) {
		return nil
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
	os.MkdirAll(s.home, 0700)

	buf := new(bytes.Buffer)
	if err := configTemplate.Execute(buf, s); err != nil {
		return err
	}

	if err := ioutil.WriteFile(path.Join(s.home, configFile), buf.Bytes(), 0700); err != nil {
		return err
	}

	if err := ioutil.WriteFile(path.Join(s.home, certFile), cert, 0700); err != nil {
		return err
	}

	if err := ioutil.WriteFile(path.Join(s.home, keyFile), key, 0700); err != nil {
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
func (s *Syncthing) Run(ctx context.Context, wg *sync.WaitGroup) error {

	if err := s.initConfig(); err != nil {
		return err
	}

	pidPath := filepath.Join(s.home, syncthingPidFile)

	if err := s.cleanupDaemon(pidPath); err != nil {
		return err
	}

	cmdArgs := []string{
		"-home", s.home,
		"-no-browser",
		"-verbose",
		"-logfile", path.Join(s.home, logFile),
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

	log.Infof("syncthing running on http://%s and tcp://%s", s.GUIAddress, s.ListenAddress)

	go func() {
		wg.Add(1)
		defer wg.Done()
		<-ctx.Done()
		if err := s.Stop(); err != nil {
			log.Info(err)
		}
		log.Debug("syncthing clean shutdown")
		return
	}()
	return nil
}

// Stop halts the background process and cleans up.
func (s *Syncthing) Stop() error {
	pidPath := filepath.Join(s.home, syncthingPidFile)

	if err := s.cleanupDaemon(pidPath); err != nil {
		return err
	}

	return nil
}

// RemoveFolder deletes all the files created by the syncthing instance
func (s *Syncthing) RemoveFolder() error {
	if s.home == "" {
		log.Info("the home directory is not set when deleting")
		return nil
	}

	if err := os.RemoveAll(s.home); err != nil {
		log.Info(err)
		return nil
	}

	parentDir := path.Dir(s.home)
	if parentDir != "." {
		empty, err := isEmpty(parentDir)
		if err != nil {
			log.Info(err)
			return nil
		}

		if empty {
			log.Debugf("deleting %s since it's empty", parentDir)
			if err := os.RemoveAll(parentDir); err != nil {
				log.Info(err)
				return nil
			}
		}
	}

	return nil
}

func isEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}

func getPID(pidPath string) (int, error) {
	if _, err := os.Stat(pidPath); err != nil {
		return 0, err
	}

	content, err := ioutil.ReadFile(pidPath) // nolint: gosec
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(string(content))
}

// Exists returns true if the syncthing process exists
func Exists(home string) bool {
	pidPath := filepath.Join(home, syncthingPidFile)
	pid, err := getPID(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}

	process, err := ps.FindProcess(pid)
	if process == nil && err == nil {
		return false
	}

	if err != nil {
		log.Infof("error when looking up the process: %s", err)
		return true
	}

	log.Debugf("found %s pid-%d ppid-%d", process.Executable(), process.Pid(), process.PPid())

	if process.Executable() == binaryName {
		return true
	}

	return false
}

// IsInstalled return true if syncthing is already installed
func IsInstalled() bool {
	_, err := os.Stat(GetInstallPath())
	if os.IsNotExist(err) {
		return false
	}

	return true
}

// GetInstallPath returns the expected install path for syncthing
func GetInstallPath() string {
	return path.Join(config.GetCNDHome(), binaryName)
}
