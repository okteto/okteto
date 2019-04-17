package syncthing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/okteto/app/cli/pkg/config"
	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/model"

	ps "github.com/mitchellh/go-ps"
)

var (
	configTemplate = template.Must(template.New("syncthingConfig").Parse(configXML))
)

const (
	certFile         = "cert.pem"
	keyFile          = "key.pem"
	configFile       = "config.xml"
	logFile          = "syncthing.log"
	syncthingPidFile = "syncthing.pid"

	// DefaultRemoteDeviceID remote syncthing ID
	DefaultRemoteDeviceID = "ATOPHFJ-VPVLDFY-QVZDCF2-OQQ7IOW-OG4DIXF-OA7RWU3-ZYA4S22-SI4XVAU"

	// DefaultFileWatcherDelay how much to wait before starting a sync after a file change
	DefaultFileWatcherDelay = 5

	// ClusterPort is the port used by syncthing in the cluster
	ClusterPort = 22000

	// GUIPort is the port used by syncthing in the cluster for the http endpoint
	GUIPort = 8384
)

// Syncthing represents the local syncthing process.
type Syncthing struct {
	APIKey           string
	binPath          string
	Client           *http.Client
	cmd              *exec.Cmd
	Dev              *model.Dev
	DevPath          string
	FileWatcherDelay int
	ForceSendOnly    bool
	GUIAddress       string
	Home             string
	ListenAddress    string
	RemoteAddress    string
	RemoteDeviceID   string
	RemoteGUIAddress string
	RemoteGUIPort    int
	RemotePort       int
	Source           string
	Type             string
}

// Status represents the status of a syncthing folder.
type Status struct {
	State string `json:"state"`
}

// Completion represents the completion status of a syncthing folder.
type Completion struct {
	Completion float64 `json:"completion"`
}

// New constructs a new Syncthing.
func New(dev *model.Dev, devPath, space string) (*Syncthing, error) {
	fullPath := GetInstallPath()
	if !IsInstalled() {
		return nil, fmt.Errorf("cannot find syncthing. Make sure syncthing is installed in %s", fullPath)
	}

	remotePort, err := model.GetAvailablePort()
	if err != nil {
		return nil, err
	}

	remoteGUIPort, err := model.GetAvailablePort()
	if err != nil {
		return nil, err
	}

	guiPort, err := model.GetAvailablePort()
	if err != nil {
		return nil, err
	}

	listenPort, err := model.GetAvailablePort()
	if err != nil {
		return nil, err
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	s := &Syncthing{
		APIKey:           "cnd",
		binPath:          fullPath,
		Client:           NewAPIClient(),
		Dev:              dev,
		DevPath:          devPath,
		FileWatcherDelay: DefaultFileWatcherDelay,
		GUIAddress:       fmt.Sprintf("127.0.0.1:%d", guiPort),
		Home:             filepath.Join(config.GetHome(), space, dev.Name),
		ListenAddress:    fmt.Sprintf("0.0.0.0:%d", listenPort),
		RemoteAddress:    fmt.Sprintf("tcp://localhost:%d", remotePort),
		RemoteDeviceID:   DefaultRemoteDeviceID,
		RemoteGUIAddress: fmt.Sprintf("localhost:%d", remoteGUIPort),
		RemoteGUIPort:    remoteGUIPort,
		RemotePort:       remotePort,
		Source:           wd,
		Type:             "sendonly",
	}

	return s, nil
}

func (s *Syncthing) cleanupDaemon(pidPath string) error {
	pid, err := getPID(pidPath)
	if os.IsNotExist(err) {
		return nil
	}

	process, err := ps.FindProcess(pid)
	if process == nil && err == nil {
		return nil
	}

	if err != nil {
		log.Infof("error when looking up the process: %s", err)
		return err
	}

	if process.Executable() != getBinaryName() {
		log.Debugf("found %s pid-%d ppid-%d", process.Executable(), process.Pid(), process.PPid())
		return nil
	}

	return terminate(pid)
}

func (s *Syncthing) initConfig() error {
	os.MkdirAll(s.Home, 0700)

	if err := s.UpdateConfig(); err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(s.Home, certFile), cert, 0700); err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(s.Home, keyFile), key, 0700); err != nil {
		return err
	}

	return nil
}

// UpdateConfig updates the synchting config file
func (s *Syncthing) UpdateConfig() error {
	buf := new(bytes.Buffer)
	if err := configTemplate.Execute(buf, s); err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(s.Home, configFile), buf.Bytes(), 0700); err != nil {
		return err
	}
	return nil
}

// Run starts up a local syncthing process to serve files from.
func (s *Syncthing) Run(ctx context.Context, wg *sync.WaitGroup) error {
	if err := s.initConfig(); err != nil {
		return err
	}

	pidPath := filepath.Join(s.Home, syncthingPidFile)

	if err := s.cleanupDaemon(pidPath); err != nil {
		return err
	}

	cmdArgs := []string{
		"-home", s.Home,
		"-no-browser",
		"-verbose",
		"-logfile", filepath.Join(s.Home, logFile),
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

// WaitForPing wait for local syncthing to ping
func (s *Syncthing) WaitForPing(ctx context.Context, wg *sync.WaitGroup) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	log.Infof("waiting for local syncthing to be ready...")
	for i := 0; i < 100; i++ {
		_, err := s.APICall("rest/system/ping", "GET", 200, nil, true)
		if err == nil {
			return nil
		}
		log.Debugf("error calling 'rest/system/ping' syncthing API: %s", err)
		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			log.Debug("cancelling call to 'rest/system/ping'")
			return ctx.Err()
		}
	}
	return fmt.Errorf("Syncthing not respondinng after 50s")
}

// WaitForScanning waits for the local syncthing to scan local folder
func (s *Syncthing) WaitForScanning(ctx context.Context, wg *sync.WaitGroup, dev *model.Dev) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	folder := fmt.Sprintf("okteto-%s", dev.Name)
	params := map[string]string{"folder": folder}
	status := &Status{}
	log.Infof("waiting for initial scan to complete...")
	for i := 0; i < 100; i++ {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			log.Debug("cancelling call to 'rest/db/status'")
			return ctx.Err()
		}

		body, err := s.APICall("rest/db/status", "GET", 200, params, true)
		if err != nil {
			log.Infof("error calling 'rest/db/status' syncthing API: %s", err)
			continue
		}
		err = json.Unmarshal(body, status)
		if err != nil {
			log.Infof("error unmarshaling 'rest/db/status': %s", err)
			continue
		}

		log.Debugf("syncthing folder is '%s'", status.State)
		if status.State != "scanning" {
			return nil
		}
	}
	return fmt.Errorf("Syncthing not completed initial scan after 50s")
}

// OverrideChanges force the remote to be the same as the local file system
func (s *Syncthing) OverrideChanges(ctx context.Context, wg *sync.WaitGroup, dev *model.Dev) error {
	folder := fmt.Sprintf("okteto-%s", dev.Name)
	params := map[string]string{"folder": folder}
	log.Infof("forcing local state to the remote container...")
	_, err := s.APICall("rest/db/override", "POST", 200, params, true)
	return err
}

// WaitForCompletion waits for the remote to be totally synched
func (s *Syncthing) WaitForCompletion(ctx context.Context, wg *sync.WaitGroup, dev *model.Dev) error {
	ticker := time.NewTicker(1 * time.Second)
	folder := fmt.Sprintf("okteto-%s", dev.Name)
	params := map[string]string{"folder": folder, "device": DefaultRemoteDeviceID}
	completion := &Completion{}
	log.Infof("waiting for synchronization to complete...")
	for true {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			log.Debug("cancelling call to 'rest/db/completion'")
			return ctx.Err()
		}

		body, err := s.APICall("rest/db/completion", "GET", 200, params, true)
		if err != nil {
			log.Infof("error calling 'rest/db/completion' syncthing API: %s", err)
			continue
		}
		err = json.Unmarshal(body, completion)
		if err != nil {
			log.Infof("error unmarshaling 'rest/db/completion': %s", err)
			continue
		}

		log.Infof("syncthing folder is '%f'", completion.Completion)
		if completion.Completion == 100 {
			return nil
		}
	}
	return nil
}

// Restart restarts the syncthing process
func (s *Syncthing) Restart(ctx context.Context, wg *sync.WaitGroup) error {
	log.Infof("restarting synchting...")
	_, err := s.APICall("rest/system/restart", "POST", 200, nil, true)
	return err
}

// Stop halts the background process and cleans up.
func (s *Syncthing) Stop() error {
	pidPath := filepath.Join(s.Home, syncthingPidFile)

	if err := s.cleanupDaemon(pidPath); err != nil {
		return err
	}

	return nil
}

// RemoveFolder deletes all the files created by the syncthing instance
func (s *Syncthing) RemoveFolder() error {
	if s.Home == "" {
		log.Info("the home directory is not set when deleting")
		return nil
	}

	if _, err := filepath.Rel(config.GetHome(), s.Home); err != nil {
		log.Debugf("%s is not inside %s, ignoring", s.Home, config.GetHome())
		return nil
	}

	if err := os.RemoveAll(s.Home); err != nil {
		log.Info(err)
		return nil
	}

	parentDir := filepath.Dir(s.Home)
	if parentDir != "." {
		empty, err := isDirEmpty(parentDir)
		if err != nil {
			log.Info(err)
			return nil
		}

		if empty {
			log.Debugf("deleting %s since it's empty", parentDir)
			if err := os.RemoveAll(parentDir); err != nil {
				log.Infof("couldn't delete folder: %s", err)
				return nil
			}
		}
	}

	return nil
}

func isDirEmpty(path string) (bool, error) {
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

	if process.Executable() == getBinaryName() {
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
	return filepath.Join(config.GetHome(), getBinaryName())
}

func getBinaryName() string {
	if runtime.GOOS == "windows" {
		return "syncthing.exe"
	}

	return "syncthing"
}
