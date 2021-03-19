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
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"golang.org/x/crypto/bcrypt"
	yaml "gopkg.in/yaml.v2"

	"github.com/google/uuid"
	gops "github.com/mitchellh/go-ps"
	"github.com/shirou/gopsutil/process"
)

var (
	configTemplate = template.Must(template.New("syncthingConfig").Parse(configXML))
)

const (
	certFile   = "cert.pem"
	keyFile    = "key.pem"
	configFile = "config.xml"
	logFile    = "syncthing.log"

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
	Folders          []*Folder    `yaml:"folders"`
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
	RescanInterval   string       `yaml:"-"`
	Compression      string       `yaml:"-"`
}

//Folder represents a sync folder
type Folder struct {
	Name         string `yaml:"name"`
	LocalPath    string `yaml:"localPath"`
	RemotePath   string `yaml:"remotePath"`
	Retries      int    `yaml:"-"`
	SentStIgnore bool   `yaml:"-"`
	Overwritten  bool   `yaml:"-"`
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
	remotePort, err := model.GetAvailablePort(dev.Interface)
	if err != nil {
		return nil, err
	}

	remoteGUIPort, err := model.GetAvailablePort(dev.Interface)
	if err != nil {
		return nil, err
	}

	guiPort, err := model.GetAvailablePort(dev.Interface)
	if err != nil {
		return nil, err
	}

	listenPort, err := model.GetAvailablePort(dev.Interface)
	if err != nil {
		return nil, err
	}

	pwd := uuid.New().String()
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), 0)
	if err != nil {
		log.Infof("couldn't hash the password %s", err)
		hash = []byte("")
	}

	compression := "metadata"
	if dev.Sync.Compression {
		compression = "always"
	}
	s := &Syncthing{
		APIKey:           "cnd",
		GUIPassword:      pwd,
		GUIPasswordHash:  string(hash),
		binPath:          fullPath,
		Client:           NewAPIClient(),
		FileWatcherDelay: DefaultFileWatcherDelay,
		GUIAddress:       fmt.Sprintf("%s:%d", dev.Interface, guiPort),
		Home:             config.GetDeploymentHome(dev.Namespace, dev.Name),
		LogPath:          GetLogFile(dev.Namespace, dev.Name),
		ListenAddress:    fmt.Sprintf("%s:%d", dev.Interface, listenPort),
		RemoteAddress:    fmt.Sprintf("tcp://%s:%d", dev.Interface, remotePort),
		RemoteDeviceID:   DefaultRemoteDeviceID,
		RemoteGUIAddress: fmt.Sprintf("%s:%d", dev.Interface, remoteGUIPort),
		LocalGUIPort:     guiPort,
		LocalPort:        listenPort,
		RemoteGUIPort:    remoteGUIPort,
		RemotePort:       remotePort,
		Type:             "sendonly",
		IgnoreDelete:     true,
		Folders:          []*Folder{},
		RescanInterval:   strconv.Itoa(dev.Sync.RescanInterval),
		Compression:      compression,
	}
	index := 1
	for _, sync := range dev.Sync.Folders {
		result, err := dev.IsSubPathFolder(sync.LocalPath)
		if err != nil {
			return nil, err
		}
		if !result {
			s.Folders = append(
				s.Folders,
				&Folder{
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

	s.pid = s.cmd.Process.Pid

	return nil
}

//WaitForPing waits for synthing to be ready
func (s *Syncthing) WaitForPing(ctx context.Context, local bool) error {
	ticker := time.NewTicker(300 * time.Millisecond)
	to := config.GetTimeout() // 30 seconds
	timeout := time.Now().Add(to)

	log.Infof("waiting for syncthing local=%t to be ready", local)
	for i := 0; ; i++ {
		if s.Ping(ctx, local) {
			return nil
		}
		if i%5 == 0 {
			log.Infof("syncthing local=%t is not ready yet", local)
		}

		if time.Now().After(timeout) {
			return fmt.Errorf("syncthing local=%t didn't respond after %s", local, to.String())
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			log.Infof("syncthing.WaitForPing cancelled local=%t", local)
			return ctx.Err()
		}
	}
}

//Ping checks if syncthing is available
func (s *Syncthing) Ping(ctx context.Context, local bool) bool {
	_, err := s.APICall(ctx, "rest/system/ping", "GET", 200, nil, local, nil, false, 1)
	if err == nil {
		return true
	}
	if strings.Contains(err.Error(), "Client.Timeout") {
		return true
	}
	return false
}

//SendStignoreFile sends .stignore from local to remote
func (s *Syncthing) SendStignoreFile(ctx context.Context) {
	for _, folder := range s.Folders {
		if folder.SentStIgnore {
			continue
		}
		log.Infof("sending '.stignore' file %s to the remote syncthing", folder.Name)
		params := getFolderParameter(folder)
		ignores := &Ignores{}
		body, err := s.APICall(ctx, "rest/db/ignores", "GET", 200, params, true, nil, true, 0)
		if err != nil {
			log.Infof("error getting ignore files: %s", err.Error())
			continue
		}
		err = json.Unmarshal(body, ignores)
		if err != nil {
			log.Infof("error unmarshalling ignore files: %s", err.Error())
			continue
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
			continue
		}
		_, err = s.APICall(ctx, "rest/db/ignores", "POST", 200, params, false, body, false, 0)
		if err != nil {
			log.Infof("error posting ignore files to remote syncthing instance: %s", err.Error())
			continue
		}
		folder.SentStIgnore = true
	}
}

//ResetDatabase resets the syncthing database
func (s *Syncthing) ResetDatabase(ctx context.Context, dev *model.Dev, local bool) error {
	for _, folder := range s.Folders {
		log.Infof("reseting syncthing database path=%s local=%t", folder.LocalPath, local)
		params := getFolderParameter(folder)
		_, err := s.APICall(ctx, "rest/system/reset", "POST", 200, params, local, nil, false, 3)
		if err != nil {
			log.Infof("error posting 'rest/system/reset' local=%t syncthing API: %s", local, err)
			if strings.Contains(err.Error(), "Client.Timeout") {
				return fmt.Errorf("error resetting syncthing database local=%t: %s", local, err.Error())
			}
			return errors.ErrLostSyncthing
		}
	}
	return nil
}

//Overwrite overwrites local changes to the remote syncthing
func (s *Syncthing) Overwrite(ctx context.Context, dev *model.Dev) error {
	for _, folder := range s.Folders {
		log.Infof("overriding local changes to the remote syncthing path=%s", folder.LocalPath)
		params := getFolderParameter(folder)
		_, err := s.APICall(ctx, "rest/db/override", "POST", 200, params, true, nil, false, 3)
		if err != nil {
			log.Infof("error posting 'rest/db/override' syncthing API: %s", err)
			if strings.Contains(err.Error(), "Client.Timeout") {
				return errors.ErrBusySyncthing
			}
			return errors.ErrLostSyncthing
		}
		folder.Overwritten = true
	}
	return nil
}

//IsAllIgnoredAndOverwritten checks if all .stignore files and overwrite operations has been completed
func (s *Syncthing) IsAllIgnoredAndOverwritten() bool {
	for _, folder := range s.Folders {
		if !folder.SentStIgnore {
			return false
		}
		if !folder.Overwritten {
			return false
		}
	}
	return true
}

//WaitForScanning waits for synthing to finish initial scanning
func (s *Syncthing) WaitForScanning(ctx context.Context, dev *model.Dev, local bool) error {
	for _, folder := range s.Folders {
		if err := s.waitForFolderScanning(ctx, folder, local); err != nil {
			return err
		}
	}
	return nil
}

func (s *Syncthing) waitForFolderScanning(ctx context.Context, folder *Folder, local bool) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	log.Infof("waiting for initial scan to complete path=%s local=%t", folder.LocalPath, local)

	to := config.GetTimeout() * 10 // 5 minutes
	timeout := time.Now().Add(to)

	for i := 0; ; i++ {
		status, err := s.GetStatus(ctx, folder, local)
		if err != nil && err != errors.ErrBusySyncthing {
			return errors.ErrUnknownSyncError
		}

		if status != nil {
			if i%100 == 0 {
				// one log every 10 seconds
				log.Infof("syncthing folder local=%t is '%s'", local, status.State)
			}
			if status.State != "scanning" && status.State != "scan-waiting" {
				return nil
			}
		}

		if time.Now().After(timeout) {
			return fmt.Errorf("initial file scan not completed after %s, please try again", to.String())
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			log.Info("call to syncthing.waitForFolderScanning canceled")
			return ctx.Err()
		}
	}
}

// WaitForCompletion waits for the remote to be totally synched
func (s *Syncthing) WaitForCompletion(ctx context.Context, dev *model.Dev, reporter chan float64) error {
	defer close(reporter)
	ticker := time.NewTicker(1000 * time.Millisecond)
	for _, folder := range s.Folders {
		log.Infof("waiting for synchronization to complete path=%s", folder.LocalPath)
		for {
			select {
			case <-ticker.C:
				s.SendStignoreFile(ctx)
				if err := s.Overwrite(ctx, dev); err != nil {
					if err == errors.ErrBusySyncthing {
						continue
					}
					return err
				}

				completion, err := s.GetCompletion(ctx, true)
				if err != nil {
					if err == errors.ErrBusySyncthing {
						continue
					}
					return err
				}

				if completion.GlobalBytes == 0 {
					if s.IsAllIgnoredAndOverwritten() {
						return nil
					}
					log.Info("synced completed, but retrying stignores and overwrites")
					continue
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

				if err := s.IsHealthy(ctx, false, 30); err != nil {
					return err
				}

			case <-ctx.Done():
				log.Info("call to syncthing.WaitForCompletion canceled")
				return ctx.Err()
			}
		}
	}
	return nil
}

// GetCompletion returns the syncthing completion
func (s *Syncthing) GetCompletion(ctx context.Context, local bool) (*Completion, error) {
	result := &Completion{}
	for _, folder := range s.Folders {
		params := getFolderParameter(folder)
		if local {
			params["device"] = DefaultRemoteDeviceID
		} else {
			params["device"] = localDeviceID
		}
		completion := &Completion{}
		body, err := s.APICall(ctx, "rest/db/completion", "GET", 200, params, local, nil, true, 3)
		if err != nil {
			log.Infof("error calling 'rest/db/completion' local=%t syncthing API: %s", local, err)
			if strings.Contains(err.Error(), "Client.Timeout") {
				return nil, errors.ErrBusySyncthing
			}
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

// IsHealthy returns the syncthing error or nil
func (s *Syncthing) IsHealthy(ctx context.Context, local bool, max int) error {
	for _, folder := range s.Folders {
		status, err := s.GetStatus(ctx, folder, false)
		if err != nil {
			if err == errors.ErrBusySyncthing {
				continue
			}
			return err
		}
		if status.PullErrors == 0 {
			folder.Retries = 0
			continue
		}

		err = s.GetFolderErrors(ctx, folder, false)
		log.Infof("syncthing error in folder '%s' local=%t: %s", folder.RemotePath, local, err)
		folder.Retries++
		if folder.Retries <= max {
			continue
		}
		if err == nil || err == errors.ErrBusySyncthing {
			return errors.ErrUnknownSyncError
		}
		return err
	}
	return nil
}

// GetStatus returns the syncthing status
func (s *Syncthing) GetStatus(ctx context.Context, folder *Folder, local bool) (*Status, error) {
	params := getFolderParameter(folder)
	status := &Status{}
	body, err := s.APICall(ctx, "rest/db/status", "GET", 200, params, local, nil, true, 3)
	if err != nil {
		log.Infof("error getting status: %s", err.Error())
		if strings.Contains(err.Error(), "Client.Timeout") {
			return nil, errors.ErrBusySyncthing
		}
		return nil, errors.ErrLostSyncthing
	}
	err = json.Unmarshal(body, status)
	if err != nil {
		log.Infof("error unmarshalling status: %s", err.Error())
		return nil, errors.ErrLostSyncthing
	}

	return status, nil
}

// GetFolderErrors returns the last folder errors
func (s *Syncthing) GetFolderErrors(ctx context.Context, folder *Folder, local bool) error {
	params := getFolderParameter(folder)
	params["since"] = "0"
	params["limit"] = "1"
	params["timeout"] = "0"
	params["events"] = "FolderErrors"
	folderErrorsList := []FolderErrors{}
	body, err := s.APICall(ctx, "rest/events", "GET", 200, params, local, nil, true, 3)
	if err != nil {
		log.Infof("error getting events: %s", err.Error())
		if strings.Contains(err.Error(), "Client.Timeout") {
			return errors.ErrBusySyncthing
		}
		return errors.ErrLostSyncthing
	}

	err = json.Unmarshal(body, &folderErrorsList)
	if err != nil {
		log.Infof("error unmarshalling events: %s", err.Error())
		return errors.ErrLostSyncthing
	}

	if len(folderErrorsList) == 0 {
		return nil
	}
	folderErrors := folderErrorsList[len(folderErrorsList)-1]
	if len(folderErrors.Data.Errors) == 0 {
		return nil
	}

	errMsg := folderErrors.Data.Errors[0].Error

	if strings.Contains(errMsg, "insufficient space") {
		log.Infof("syncthing insufficient space local=%t: %s", local, errMsg)
		return errors.ErrInsufficientSpace
	}

	log.Infof("syncthing pull error local=%t: %s", local, errMsg)
	return fmt.Errorf("%s: %s", folderErrors.Data.Errors[0].Path, errMsg)
}

// Restart restarts the syncthing process
func (s *Syncthing) Restart(ctx context.Context) error {
	_, err := s.APICall(ctx, "rest/system/restart", "POST", 200, nil, true, nil, false, 3)
	return err
}

// HardTerminate halts the background process, waits for 1s and kills the process if it is still running
func (s *Syncthing) HardTerminate() error {
	pList, err := process.Processes()
	if err != nil {
		return err
	}

	for _, p := range pList {
		if p.Pid == 0 {
			continue
		}

		name, err := p.Name()
		if err != nil {
			// it's expected go get EOF if the process no longer exists at this point.
			if err != io.EOF {
				log.Infof("error getting name for process %d: %s", p.Pid, err.Error())
			}
			continue
		}

		// workaround until https://github.com/shirou/gopsutil/issues/1043 is fixed
		if name == "" && runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
			pr, err := gops.FindProcess(int(p.Pid))
			if err != nil {
				log.Infof("error getting process  %d: %s", p.Pid, err.Error())
				continue
			}

			if pr == nil {
				log.Infof("process  %d not found", p.Pid)
				continue
			}

			name = pr.Executable()
		}

		if name == "" {
			continue
		}

		if !strings.Contains(name, "syncthing") {
			continue
		}

		cmdline, err := p.Cmdline()
		if err != nil {
			return err
		}

		if !strings.Contains(cmdline, fmt.Sprintf("-home %s", s.Home)) {
			continue
		}
		log.Infof("terminating syncthing %d with wait: %s", p.Pid, s.Home)
		if err := terminate(p, true); err != nil {
			log.Infof("error terminating syncthing %d with wait: %s", p.Pid, err.Error())
		}
		log.Infof("terminated syncthing %d with wait: %s", p.Pid, s.Home)
	}
	return nil
}

// SoftTerminate halts the background process
func (s *Syncthing) SoftTerminate() error {
	if s.pid == 0 {
		return nil
	}
	p, err := process.NewProcess(int32(s.pid))
	if err != nil {
		return fmt.Errorf("error getting syncthing process %d: %s", s.pid, err.Error())
	}
	log.Infof("terminating syncthing %d without wait", s.pid)
	if err := terminate(p, false); err != nil {
		return fmt.Errorf("error terminating syncthing %d without wait: %s", p.Pid, err.Error())
	}
	log.Infof("terminated syncthing %d without wait", s.pid)
	return nil
}

// SaveConfig saves the syncthing object in the dev home folder
func (s *Syncthing) SaveConfig(dev *model.Dev) error {
	marshalled, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	syncthingInfoFile := getInfoFile(dev.Namespace, dev.Name)
	if err := ioutil.WriteFile(syncthingInfoFile, marshalled, 0600); err != nil {
		return fmt.Errorf("failed to write syncthing info file: %w", err)
	}

	return nil
}

// Load loads the syncthing object from the dev home folder
func Load(dev *model.Dev) (*Syncthing, error) {
	syncthingInfoFile := getInfoFile(dev.Namespace, dev.Name)
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
			if err := os.RemoveAll(parentDir); err != nil {
				log.Infof("couldn't delete folder: %s", err)
				return nil
			}

			log.Infof("removed %s", parentDir)
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

func getInfoFile(namespace, name string) string {
	return filepath.Join(config.GetDeploymentHome(namespace, name), "syncthing.info")
}

// GetLogFile returns the path to the syncthing log file
func GetLogFile(namespace, name string) string {
	return filepath.Join(config.GetDeploymentHome(namespace, name), "syncthing.log")
}
