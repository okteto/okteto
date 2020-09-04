// Copyright 2020 The Okteto Authors
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
	"strings"
	"text/template"
	"time"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	okerr "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"golang.org/x/crypto/bcrypt"
	yaml "gopkg.in/yaml.v2"

	"github.com/google/uuid"
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
	localDeviceID         = "ABKAVQF-RUO4CYO-FSC2VIP-VRX4QDA-TQQRN2J-MRDXJUC-FXNWP6N-S6ZSAAR"

	// DefaultFileWatcherDelay how much to wait before starting a sync after a file change
	DefaultFileWatcherDelay = 5

	// ClusterPort is the port used by syncthing in the cluster
	ClusterPort = 22000

	// GUIPort is the port used by syncthing in the cluster for the http endpoint
	GUIPort = 8384
)

// Syncthing represents the local syncthing process.
type Syncthing struct {
	APIKey           string       `yaml:"apikey"`
	GUIPassword      string       `yaml:"password"`
	GUIPasswordHash  string       `yaml:"-"`
	binPath          string       `yaml:"-"`
	Client           *http.Client `yaml:"-"`
	cmd              *exec.Cmd    `yaml:"-"`
	Folders          []Folder     `yaml:"folders"`
	FileWatcherDelay int          `yaml:"-"`
	ForceSendOnly    bool         `yaml:"-"`
	GUIAddress       string       `yaml:"local"`
	Home             string       `yaml:"-"`
	LogPath          string       `yaml:"-"`
	ListenAddress    string       `yaml:"-"`
	RemoteAddress    string       `yaml:"-"`
	RemoteDeviceID   string       `yaml:"-"`
	RemoteGUIAddress string       `yaml:"remote"`
	RemoteGUIPort    int          `yaml:"-"`
	RemotePort       int          `yaml:"-"`
	LocalGUIPort     int          `yaml:"-"`
	LocalPort        int          `yaml:"-"`
	Type             string       `yaml:"-"`
	IgnoreDelete     bool         `yaml:"-"`
	pid              int          `yaml:"-"`
}

//Folder represents a sync folder
type Folder struct {
	Name       string `yaml:"name"`
	LocalPath  string `yaml:"localPath"`
	RemotePath string `yaml:"remotePath"`
}

//Ignores represents the .stignore file
type Ignores struct {
	Ignore []string `json:"ignore"`
}

// Status represents the status of a syncthing folder.
type Status struct {
	State      string `json:"state"`
	PullErrors int64  `json:"pullErrors"`
}

// Completion represents the completion of a syncthing folder.
type Completion struct {
	Completion  float64 `json:"completion"`
	GlobalBytes int64   `json:"globalBytes"`
	NeedBytes   int64   `json:"needBytes"`
	NeedDeletes int64   `json:"needDeletes"`
}

// FolderErrors represents folder errors in syncthing.
type FolderErrors struct {
	Data DataFolderErrors `json:"data"`
}

// DataFolderErrors represents data folder errors in syncthing.
type DataFolderErrors struct {
	Errors []FolderError `json:"errors"`
}

// FolderError represents a folder error in syncthing.
type FolderError struct {
	Error string `json:"error"`
	Path  string `json:"path"`
}

// New constructs a new Syncthing.
func New(dev *model.Dev) (*Syncthing, error) {
	fullPath := getInstallPath()
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

	pwd := uuid.New().String()
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
		FileWatcherDelay: DefaultFileWatcherDelay,
		GUIAddress:       fmt.Sprintf("localhost:%d", guiPort),
		Home:             config.GetDeploymentHome(dev.Namespace, dev.Name),
		LogPath:          config.GetSyncthingLogFile(dev.Namespace, dev.Name),
		ListenAddress:    fmt.Sprintf("localhost:%d", listenPort),
		RemoteAddress:    fmt.Sprintf("tcp://localhost:%d", remotePort),
		RemoteDeviceID:   DefaultRemoteDeviceID,
		RemoteGUIAddress: fmt.Sprintf("localhost:%d", remoteGUIPort),
		LocalGUIPort:     guiPort,
		LocalPort:        listenPort,
		RemoteGUIPort:    remoteGUIPort,
		RemotePort:       remotePort,
		Type:             "sendonly",
		IgnoreDelete:     true,
		Folders:          []Folder{},
	}
	index := 1
	for _, sync := range dev.Syncs {
		result, err := dev.IsSubPathFolder(sync.LocalPath)
		if err != nil {
			return nil, err
		}
		if !result {
			s.Folders = append(
				s.Folders,
				Folder{
					Name:       strconv.Itoa(index),
					LocalPath:  sync.LocalPath,
					RemotePath: sync.RemotePath,
				},
			)
			index++
		}
	}

	return s, nil
}

func (s *Syncthing) cleanupDaemon(pid int, wait bool) error {
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

	err = terminate(pid, wait)
	if err == nil {
		log.Infof("terminated syncthing with pid %d", pid)
	}

	return err
}

func (s *Syncthing) initConfig() error {
	if err := os.MkdirAll(s.Home, 0700); err != nil {
		return fmt.Errorf("failed to create %s: %s", s.Home, err)
	}

	if err := s.UpdateConfig(); err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(s.Home, certFile), cert, 0700); err != nil {
		return fmt.Errorf("failed to write syncthing certificate: %w", err)
	}

	if err := ioutil.WriteFile(filepath.Join(s.Home, keyFile), key, 0700); err != nil {
		return fmt.Errorf("failed to write syncthing key: %w", err)
	}

	return nil
}

// UpdateConfig updates the syncthing config file
func (s *Syncthing) UpdateConfig() error {
	buf := new(bytes.Buffer)
	if err := configTemplate.Execute(buf, s); err != nil {
		return fmt.Errorf("failed to write syncthing configuration template: %w", err)
	}

	if err := ioutil.WriteFile(filepath.Join(s.Home, configFile), buf.Bytes(), 0700); err != nil {
		return fmt.Errorf("failed to write syncthing configuration file: %w", err)
	}

	return nil
}

// Run starts up a local syncthing process to serve files from.
func (s *Syncthing) Run(ctx context.Context) error {
	if err := s.initConfig(); err != nil {
		return err
	}

	pidPath := filepath.Join(s.Home, syncthingPidFile)

	cmdArgs := []string{
		"-home", s.Home,
		"-no-browser",
		"-verbose",
		"-logfile", s.LogPath,
		"-log-max-old-files=0",
	}

	s.cmd = exec.Command(s.binPath, cmdArgs...) //nolint: gas, gosec
	s.cmd.Env = append(os.Environ(), "STNOUPGRADE=1")

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start syncthing: %w", err)
	}

	if s.cmd.Process == nil {
		return nil
	}

	if err := ioutil.WriteFile(pidPath, []byte(strconv.Itoa(s.cmd.Process.Pid)), 0600); err != nil {
		return fmt.Errorf("failed to write syncthing pid file: %w", err)
	}

	s.pid = s.cmd.Process.Pid

	log.Infof("local syncthing pid-%d running", s.pid)
	return nil
}

//WaitForPing waits for synthing to be ready
func (s *Syncthing) WaitForPing(ctx context.Context, local bool) error {
	ticker := time.NewTicker(300 * time.Millisecond)
	to := config.GetTimeout() // 30 seconds
	timeout := time.Now().Add(to)

	log.Infof("waiting for syncthing local=%t to be ready", local)
	for i := 0; ; i++ {
		_, err := s.APICall(ctx, "rest/system/ping", "GET", 200, nil, local, nil, false)
		if err == nil {
			log.Infof("syncthing local=%t is ready", local)
			return nil
		}

		if i%5 == 0 {
			log.Debugf("syncthing local=%t is not ready yet: %s", local, err)
		}

		if time.Now().After(timeout) {
			return fmt.Errorf("syncthing local=%t didn't respond after %s", local, to.String())
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			log.Debugf("cancelling syncthing.WaitForPing local=%t", local)
			return ctx.Err()
		}
	}

}

//SendStignoreFile sends .stignore from local to remote
func (s *Syncthing) SendStignoreFile(ctx context.Context, dev *model.Dev) error {
	for _, folder := range s.Folders {
		log.Infof("sending '.stignore' file %s to the remote syncthing", folder.Name)
		params := getFolderParameter(&folder)
		ignores := &Ignores{}
		body, err := s.APICall(ctx, "rest/db/ignores", "GET", 200, params, true, nil, true)
		if err != nil {
			log.Infof("error getting ignore files: %s", err.Error())
			return errors.ErrLostSyncthing
		}
		err = json.Unmarshal(body, ignores)
		if err != nil {
			return fmt.Errorf("error unmarshalling ignore files: %s", err.Error())
		}
		for i, line := range ignores.Ignore {
			line := strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.Contains(line, "(?d)") {
				continue
			}
			ignores.Ignore[i] = fmt.Sprintf("(?d)%s", line)
		}
		body, err = json.Marshal(ignores)
		if err != nil {
			log.Infof("error marshalling ignore files: %s", err.Error())
			return errors.ErrLostSyncthing
		}
		_, err = s.APICall(ctx, "rest/db/ignores", "POST", 200, params, false, body, false)
		if err != nil {
			log.Infof("error posting ignore files: %s", err.Error())
			return errors.ErrLostSyncthing
		}
	}
	return nil
}

//ResetDatabase resets the syncthing database
func (s *Syncthing) ResetDatabase(ctx context.Context, dev *model.Dev, local bool) error {
	for _, folder := range s.Folders {
		log.Infof("reseting syncthing database path=%s local=%t", folder.LocalPath, local)
		params := getFolderParameter(&folder)
		_, err := s.APICall(ctx, "rest/system/reset", "POST", 200, params, local, nil, false)
		if err != nil {
			log.Infof("error posting 'rest/system/reset' local=%t syncthing API: %s", local, err)
			return err
		}
	}
	return nil
}

//Overwrite overwrites local changes to the remote syncthing
func (s *Syncthing) Overwrite(ctx context.Context, dev *model.Dev) error {
	for _, folder := range s.Folders {
		log.Infof("overriding local changes to the remote syncthing path=%s", folder.LocalPath)
		params := getFolderParameter(&folder)
		_, err := s.APICall(ctx, "rest/db/override", "POST", 200, params, true, nil, false)
		if err != nil {
			log.Infof("error posting 'rest/db/override' syncthing API: %s", err)
			return errors.ErrLostSyncthing
		}
	}
	return nil
}

//WaitForScanning waits for synthing to finish initial scanning
func (s *Syncthing) WaitForScanning(ctx context.Context, dev *model.Dev, local bool) error {
	for _, folder := range s.Folders {
		if err := s.waitForFolderScanning(ctx, &folder, local); err != nil {
			return err
		}
	}
	return nil
}

func (s *Syncthing) waitForFolderScanning(ctx context.Context, folder *Folder, local bool) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	params := getFolderParameter(folder)
	status := &Status{}
	log.Infof("waiting for initial scan to complete path=%s local=%t", folder.LocalPath, local)

	to := config.GetTimeout() * 10 // 5 minutes
	timeout := time.Now().Add(to)

	for i := 0; ; i++ {
		body, err := s.APICall(ctx, "rest/db/status", "GET", 200, params, local, nil, true)
		if err != nil {
			log.Infof("error calling 'rest/db/status' local=%t syncthing API: %s", local, err)
			return errors.ErrLostSyncthing
		}
		err = json.Unmarshal(body, status)
		if err != nil {
			log.Infof("error unmarshaling 'rest/db/status': %s", err)
			return errors.ErrLostSyncthing
		}

		if i%100 == 0 {
			// one log every 10 seconds
			log.Infof("syncthing folder local=%t is '%s'", local, status.State)
		}

		if status.State != "scanning" && status.State != "scan-waiting" {
			return nil
		}

		if time.Now().After(timeout) {
			return fmt.Errorf("initial file scan not completed after %s, please try again", to.String())
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			log.Debug("cancelling call to 'rest/db/status'")
			return ctx.Err()
		}
	}
}

// WaitForCompletion waits for the remote to be totally synched
func (s *Syncthing) WaitForCompletion(ctx context.Context, dev *model.Dev, reporter chan float64) error {
	defer close(reporter)
	ticker := time.NewTicker(500 * time.Millisecond)
	for _, folder := range s.Folders {
		log.Infof("waiting for synchronization to complete path=%s", folder.LocalPath)
		retries := 0
		for {
			select {
			case <-ticker.C:
				if err := s.Overwrite(ctx, dev); err != nil {
					return err
				}

				completion, err := s.GetCompletion(ctx, true)
				if err != nil {
					return err
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

				for _, folder := range s.Folders {
					status, err := s.GetStatus(ctx, &folder, false)
					if err != nil {
						return err
					}
					if status.PullErrors > 0 {
						if err := s.GetFolderErrors(ctx, &folder, false); err != nil {
							return err
						}
						retries++
						if retries >= 60 {
							return okerr.ErrUnknownSyncError
						}
						continue
					}
					retries = 0
				}

			case <-ctx.Done():
				log.Debug("cancelling call to 'rest/db/completion'")
				return ctx.Err()
			}
		}
	}
	return nil
}

// GetStatus returns the syncthing status
func (s *Syncthing) GetStatus(ctx context.Context, folder *Folder, local bool) (*Status, error) {
	params := getFolderParameter(folder)
	status := &Status{}
	body, err := s.APICall(ctx, "rest/db/status", "GET", 200, params, local, nil, true)
	if err != nil {
		log.Infof("error getting status: %s", err.Error())
		return nil, errors.ErrLostSyncthing
	}
	err = json.Unmarshal(body, status)
	if err != nil {
		log.Infof("error unmarshalling status: %s", err.Error())
		return nil, errors.ErrLostSyncthing
	}

	return status, nil
}

// GetCompletion returns the syncthing completion
func (s *Syncthing) GetCompletion(ctx context.Context, local bool) (*Completion, error) {
	result := &Completion{}
	for _, folder := range s.Folders {
		params := getFolderParameter(&folder)
		if local {
			params["device"] = DefaultRemoteDeviceID
		} else {
			params["device"] = localDeviceID
		}
		completion := &Completion{}
		body, err := s.APICall(ctx, "rest/db/completion", "GET", 200, params, local, nil, true)
		if err != nil {
			log.Infof("error calling 'rest/db/completion' local=%t syncthing API: %s", local, err)
			return nil, errors.ErrLostSyncthing
		}
		err = json.Unmarshal(body, completion)
		if err != nil {
			log.Infof("error unmarshalling 'rest/db/completion' local=%t syncthing API: %s", local, err)
			return nil, errors.ErrLostSyncthing
		}
		result.Completion += completion.Completion
		result.GlobalBytes += completion.GlobalBytes
		result.NeedBytes += completion.NeedBytes
		result.NeedDeletes += completion.NeedDeletes
	}

	return result, nil
}

// GetCompletionProgress returns the syncthing completion progress
func (s *Syncthing) GetCompletionProgress(ctx context.Context, local bool) (float64, error) {
	completion, err := s.GetCompletion(ctx, local)
	if err != nil {
		return 0, err
	}
	if completion.GlobalBytes == 0 {
		return 100, nil
	}
	progress := (float64(completion.GlobalBytes-completion.NeedBytes) / float64(completion.GlobalBytes)) * 100
	return progress, nil
}

// GetFolderErrors returns the last folder errors
func (s *Syncthing) GetFolderErrors(ctx context.Context, folder *Folder, local bool) error {
	params := getFolderParameter(folder)
	params["since"] = "0"
	params["limit"] = "1"
	params["timeout"] = "15"
	params["events"] = "FolderErrors"
	folderErrorsList := []FolderErrors{}
	body, err := s.APICall(ctx, "rest/events", "GET", 200, params, local, nil, true)
	if err != nil {
		log.Infof("error getting events: %s", err.Error())
		return errors.ErrLostSyncthing
	}
	err = json.Unmarshal(body, &folderErrorsList)
	if err != nil {
		log.Infof("error unmarshalling events: %s", err.Error())
		return errors.ErrLostSyncthing
	}

	if len(folderErrorsList) == 0 {
		log.Infof("ignoring syncthing unknown error local=%t: empty folderErrorsList", local)
		return nil
	}
	folderErrors := folderErrorsList[len(folderErrorsList)-1]
	if len(folderErrors.Data.Errors) == 0 {
		log.Infof("ignoring syncthing unknown error local=%t: empty folderErrors.Data.Errors", local)
		return nil
	}

	errMsg := folderErrors.Data.Errors[0].Error
	if strings.Contains(errMsg, "too many open files") {
		log.Infof("ignoring syncthing 'too many open files' error local=%t: %s", local, errMsg)
		return nil
	}

	if strings.Contains(errMsg, "no connected device has the required version of this file") {
		log.Infof("corrupted syncthing database, needs reset local=%t: %s", local, errMsg)
		return errors.ErrResetSyncthing
	}

	log.Infof("syncthing pull error local=%t: %s", local, errMsg)
	return fmt.Errorf("%s: %s", folderErrors.Data.Errors[0].Path, errMsg)
}

// Restart restarts the syncthing process
func (s *Syncthing) Restart(ctx context.Context) error {
	_, err := s.APICall(ctx, "rest/system/restart", "POST", 200, nil, true, nil, false)
	return err
}

// Stop halts the background process and cleans up.
func (s *Syncthing) Stop(force bool) error {
	pidPath := filepath.Join(s.Home, syncthingPidFile)
	pid, err := getPID(pidPath)
	if os.IsNotExist(err) {
		return nil
	}

	if !force {
		if pid != s.pid {
			log.Infof("syncthing pid-%d wasn't created by this command, skipping", pid)
			return nil
		}
	}

	if err := s.cleanupDaemon(pid, force); err != nil {
		return err
	}

	if err := os.Remove(pidPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		log.Infof("failed to delete pidfile %s: %s", pidPath, err)
	}

	return nil
}

// SaveConfig saves the syncthing object in the dev home folder
func (s *Syncthing) SaveConfig(dev *model.Dev) error {
	marshalled, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	syncthingInfoFile := config.GetSyncthingInfoFile(dev.Namespace, dev.Name)
	if err := ioutil.WriteFile(syncthingInfoFile, marshalled, 0600); err != nil {
		return fmt.Errorf("failed to write syncthing info file: %w", err)
	}

	return nil
}

// Load loads the syncthing object from the dev home folder
func Load(dev *model.Dev) (*Syncthing, error) {
	syncthingInfoFile := config.GetSyncthingInfoFile(dev.Namespace, dev.Name)
	b, err := ioutil.ReadFile(syncthingInfoFile)
	if err != nil {
		return nil, err
	}

	s := &Syncthing{
		Client: NewAPIClient(),
	}
	if err := yaml.Unmarshal(b, s); err != nil {
		return nil, err
	}

	return s, nil
}

// RemoveFolder deletes all the files created by the syncthing instance
func RemoveFolder(dev *model.Dev) error {
	s, err := New(dev)
	if err != nil {
		return fmt.Errorf("failed to create syncthing instance")
	}

	if s.Home == "" {
		log.Info("the home directory is not set when deleting")
		return nil
	}

	if _, err := filepath.Rel(config.GetOktetoHome(), s.Home); err != nil || config.GetOktetoHome() == s.Home {
		log.Errorf("%s is not inside %s, ignoring", s.Home, config.GetOktetoHome())
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

func getInstallPath() string {
	return filepath.Join(config.GetOktetoHome(), getBinaryName())
}

func getBinaryName() string {
	if runtime.GOOS == "windows" {
		return "syncthing.exe"
	}

	return "syncthing"
}

func getFolderParameter(folder *Folder) map[string]string {
	folderName := fmt.Sprintf("okteto-%s", folder.Name)
	return map[string]string{"folder": folderName, "device": DefaultRemoteDeviceID}
}
