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

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"golang.org/x/crypto/bcrypt"

	ps "github.com/mitchellh/go-ps"
	uuid "github.com/satori/go.uuid"
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
	GUIPassword      string
	GUIPasswordHash  string
	binPath          string
	Client           *http.Client
	cmd              *exec.Cmd
	Dev              *model.Dev
	DevPath          string
	FileWatcherDelay int
	ForceSendOnly    bool
	GUIAddress       string
	Home             string
	LogPath          string
	ListenAddress    string
	RemoteAddress    string
	RemoteDeviceID   string
	RemoteGUIAddress string
	RemoteGUIPort    int
	RemotePort       int
	Source           string
	Type             string
}

//Ignores represents the .stignore file
type Ignores struct {
	Ignore []string `json:"ignore"`
}

// Status represents the status of a syncthing folder.
type Status struct {
	State string `json:"state"`
}

// Completion represents the completion status of a syncthing folder.
type Completion struct {
	Completion  float64 `json:"completion"`
	GlobalBytes int64   `json:"globalBytes"`
	NeedBytes   int64   `json:"needBytes"`
	NeedDeletes int64   `json:"needDeletes"`
}

// New constructs a new Syncthing.
func New(dev *model.Dev) (*Syncthing, error) {
	fullPath := GetInstallPath()
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

	pwd := uuid.NewV4().String()
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), 0)
	if err != nil {
		log.Infof("couldn't hash the password %s", err)
		hash = []byte("")
	}

	s := &Syncthing{
		APIKey:           "cnd",
		GUIPassword:      pwd,
		GUIPasswordHash:  string(hash),
		binPath:          fullPath,
		Client:           NewAPIClient(),
		Dev:              dev,
		DevPath:          dev.DevPath,
		FileWatcherDelay: DefaultFileWatcherDelay,
		GUIAddress:       fmt.Sprintf("localhost:%d", guiPort),
		Home:             config.GetDeploymentHome(dev.Namespace, dev.Name),
		LogPath:          filepath.Join(config.GetDeploymentHome(dev.Namespace, dev.Name), logFile),
		ListenAddress:    fmt.Sprintf("localhost:%d", listenPort),
		RemoteAddress:    fmt.Sprintf("tcp://localhost:%d", remotePort),
		RemoteDeviceID:   DefaultRemoteDeviceID,
		RemoteGUIAddress: fmt.Sprintf("localhost:%d", remoteGUIPort),
		RemoteGUIPort:    remoteGUIPort,
		RemotePort:       remotePort,
		Source:           dev.DevDir,
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
	if err := os.MkdirAll(s.Home, 0700); err != nil {
		return fmt.Errorf("failed to create %s: %s", s.Home, err)
	}

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
		"-logfile", s.LogPath,
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

	log.Infof("syncthing running on http://%s and tcp://%s with password '%s'", s.GUIAddress, s.ListenAddress, s.GUIPassword)

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		if err := s.Stop(); err != nil {
			log.Info(err)
		}
		log.Debug("syncthing clean shutdown")
	}()
	return nil
}

//WaitForPing waits for synthing to be ready
func (s *Syncthing) WaitForPing(ctx context.Context, wg *sync.WaitGroup, local bool) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	log.Infof("waiting for syncthing to be ready...")
	for i := 0; i < 200; i++ {
		_, err := s.APICall("rest/system/ping", "GET", 200, nil, local, nil)
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
	return fmt.Errorf("Syncthing not responding after 50s")
}

//SendStignoreFile sends .stignore from local to remote
func (s *Syncthing) SendStignoreFile(ctx context.Context, wg *sync.WaitGroup, dev *model.Dev) {
	log.Infof("Sending '.stignore' file to the remote container...")
	folder := fmt.Sprintf("okteto-%s", dev.Name)
	params := map[string]string{"folder": folder}
	ignores := &Ignores{}
	body, err := s.APICall("rest/db/ignores", "GET", 200, params, true, nil)
	if err != nil {
		log.Infof("error getting 'rest/db/ignores' syncthing API: %s", err)
		return
	}
	err = json.Unmarshal(body, ignores)
	if err != nil {
		log.Infof("error unmarshaling 'rest/db/ignores': %s", err)
		return
	}
	body, err = json.Marshal(ignores)
	if err != nil {
		log.Infof("error marshaling 'rest/db/ignores': %s", err)
	}
	_, err = s.APICall("rest/db/ignores", "POST", 200, params, false, body)
	if err != nil {
		log.Infof("error posting 'rest/db/ignores' syncthing API: %s", err)
		return
	}
}

//WaitForScanning waits for synthing to finish initial scanning
func (s *Syncthing) WaitForScanning(ctx context.Context, wg *sync.WaitGroup, dev *model.Dev, local bool) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	folder := fmt.Sprintf("okteto-%s", dev.Name)
	params := map[string]string{"folder": folder}
	status := &Status{}
	log.Infof("waiting for initial scan to complete...")
	for i := 0; i < 480; i++ {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			log.Debug("cancelling call to 'rest/db/status'")
			return ctx.Err()
		}

		body, err := s.APICall("rest/db/status", "GET", 200, params, local, nil)
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
	return fmt.Errorf("Syncthing not completed initial scan after 2min. Please, retry in a few minutes")
}

// WaitForCompletion waits for the remote to be totally synched
func (s *Syncthing) WaitForCompletion(ctx context.Context, wg *sync.WaitGroup, dev *model.Dev, reporter chan float64) error {
	defer close(reporter)
	ticker := time.NewTicker(500 * time.Millisecond)
	folder := fmt.Sprintf("okteto-%s", dev.Name)
	params := map[string]string{"folder": folder, "device": DefaultRemoteDeviceID}
	completion := &Completion{}
	var prevNeedBytes int64
	needZeroBytesIter := 0
	log.Infof("waiting for synchronization to complete...")
	for {
		select {
		case <-ticker.C:
			if prevNeedBytes == completion.NeedBytes {
				if needZeroBytesIter >= 50 {
					return errors.ErrSyncFrozen
				}

				needZeroBytesIter++
			} else {
				needZeroBytesIter = 0
			}
		case <-ctx.Done():
			log.Debug("cancelling call to 'rest/db/completion'")
			return ctx.Err()
		}

		if _, err := s.APICall("rest/db/override", "POST", 200, params, true, nil); err != nil {
			//overwrite on each iteration to avoid sude effects of remote scannings
			log.Infof("error calling 'rest/db/override' syncthing API: %s", err)
		}

		prevNeedBytes = completion.NeedBytes
		body, err := s.APICall("rest/db/completion", "GET", 200, params, true, nil)
		if err != nil {
			log.Infof("error calling 'rest/db/completion' syncthing API: %s", err)
			continue
		}
		err = json.Unmarshal(body, completion)
		if err != nil {
			log.Infof("error unmarshaling 'rest/db/completion': %s", err)
			continue
		}

		if completion.GlobalBytes == 0 {
			return nil
		}

		progress := (float64(completion.GlobalBytes-completion.NeedBytes) / float64(completion.GlobalBytes)) * 100
		log.Infof("syncthing folder is %.2f%%, needBytes %d, needDeletes %d",
			progress,
			completion.NeedBytes,
			completion.NeedDeletes,
		)

		reporter <- progress

		if completion.NeedBytes == 0 {
			return nil
		}
	}
}

// Restart restarts the syncthing process
func (s *Syncthing) Restart(ctx context.Context, wg *sync.WaitGroup) error {
	log.Infof("restarting synchting...")
	_, err := s.APICall("rest/system/restart", "POST", 200, nil, true, nil)
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

	return process.Executable() == getBinaryName()
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
