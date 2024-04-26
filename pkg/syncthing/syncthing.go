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

package syncthing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/shirou/gopsutil/process"
	"golang.org/x/crypto/bcrypt"
	yaml "gopkg.in/yaml.v2"
)

var (
	configTemplate     = template.Must(template.New("syncthingConfig").Parse(configXML))
	isHealthyRetries   = 0
	cachedStatus       = map[bool]string{}
	cachedPullErrors   = map[bool]int64{}
	cachedFolderErrors = map[bool]FolderErrorEvent{}
)

const (
	certFile   = "cert.pem"
	keyFile    = "key.pem"
	configFile = "config.xml"

	// DefaultRemoteDeviceID remote syncthing device ID
	DefaultRemoteDeviceID = "ATOPHFJ-VPVLDFY-QVZDCF2-OQQ7IOW-OG4DIXF-OA7RWU3-ZYA4S22-SI4XVAU"
	// LocalDeviceID local syncthing device ID
	LocalDeviceID = "ABKAVQF-RUO4CYO-FSC2VIP-VRX4QDA-TQQRN2J-MRDXJUC-FXNWP6N-S6ZSAAR"

	// DefaultFileWatcherDelay how much to wait before starting a sync after a file change
	DefaultFileWatcherDelay = 5

	// ClusterPort is the port used by syncthing in the cluster
	ClusterPort = 22000

	// GUIPort is the port used by syncthing in the cluster for the http endpoint
	GUIPort = 8384

	maxRetries = 3
)

// Syncthing represents the local syncthing process.
type Syncthing struct {
	Client           *http.Client  `yaml:"-"`
	cmd              *exec.Cmd     `yaml:"-"`
	Type             string        `yaml:"-"`
	APIKey           string        `yaml:"apikey"`
	RemoteDeviceID   string        `yaml:"-"`
	RemoteGUIAddress string        `yaml:"remote"`
	GUIPassword      string        `yaml:"password"`
	GUIPasswordHash  string        `yaml:"-"`
	binPath          string        `yaml:"-"`
	GUIAddress       string        `yaml:"local"`
	Home             string        `yaml:"-"`
	LogPath          string        `yaml:"-"`
	ListenAddress    string        `yaml:"-"`
	RemoteAddress    string        `yaml:"-"`
	RescanInterval   string        `yaml:"-"`
	Compression      string        `yaml:"-"`
	Folders          []*Folder     `yaml:"folders"`
	timeout          time.Duration `yaml:"-"`
	FileWatcherDelay int           `yaml:"-"`
	RemoteGUIPort    int           `yaml:"-"`
	RemotePort       int           `yaml:"-"`
	LocalGUIPort     int           `yaml:"-"`
	LocalPort        int           `yaml:"-"`
	pid              int           `yaml:"-"`
	ForceSendOnly    bool          `yaml:"-"`
	ResetDatabase    bool          `yaml:"-"`
	IgnoreDelete     bool          `yaml:"-"`
	Verbose          bool          `yaml:"-"`
}

// Folder represents a sync folder
type Folder struct {
	Name        string `yaml:"name"`
	LocalPath   string `yaml:"localPath"`
	RemotePath  string `yaml:"remotePath"`
	Overwritten bool   `yaml:"-"`
}

// Status represents the status of a syncthing folder.
type Status struct {
	State      string `json:"state"`
	PullErrors int64  `json:"pullErrors"`
}

// StateChangedEvent represents state changed in syncthing.
type StateChangedEvent struct {
	Type string                `json:"type"`
	Data DataStateChangedEvent `json:"data"`
}

// DataStateChangedEvent represents data state changed in syncthing.
type DataStateChangedEvent struct {
	State string `json:"to"`
}

// FolderSummaryEvent represents folder summary in syncthing.
type FolderSummaryEvent struct {
	Type string                 `json:"type"`
	Data DataFolderSummaryEvent `json:"data"`
}

// DataFolderSummaryEvent represents data folder summary in syncthing.
type DataFolderSummaryEvent struct {
	Summary Status `json:"summary"`
}

// FolderErrorEvent represents folder errors in syncthing.
type FolderErrorEvent struct {
	Type string               `json:"type"`
	Data DataFolderErrorEvent `json:"data"`
}

// DataFolderErrorEvent represents data folder errors in syncthing.
type DataFolderErrorEvent struct {
	Errors []FolderError `json:"errors"`
}

// FolderError represents a folder error in syncthing.
type FolderError struct {
	Error string `json:"error"`
	Path  string `json:"path"`
}

// ItemEvent represents an item event of any type in syncthing.
type ItemEvent struct {
	Time     time.Time                                  `json:"time"`
	Data     map[string]map[string]DownloadProgressData `json:"data"`
	Id       int                                        `json:"id"`
	GlobalId int                                        `json:"globalID"`
}

// SystemError represents a system error in syncthing.
type SystemError struct {
	Message string `json:"message"`
}

// SystemErrors represents a list of system errors
type SystemErrors struct {
	Errors []SystemError `json:"errors"`
}

// Connections represents syncthing connections.
type Connections struct {
	Connections map[string]Connection `json:"connections"`
}

// Connection represents syncthing connection.
type Connection struct {
	Connected bool `json:"connected"`
}

// DownloadProgressData represents an the information about a DownloadProgress event
type DownloadProgressData struct {
	BytesTotal int64 `json:"bytesTotal"`
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
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), 10)
	if err != nil {
		oktetoLog.Infof("couldn't hash the password %s", err)
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
		GUIAddress:       net.JoinHostPort(dev.Interface, strconv.Itoa(guiPort)),
		Home:             config.GetAppHome(dev.Namespace, dev.Name),
		LogPath:          GetLogFile(dev.Namespace, dev.Name),
		ListenAddress:    net.JoinHostPort(dev.Interface, strconv.Itoa(listenPort)),
		RemoteAddress:    fmt.Sprintf("tcp://%s:%d", dev.Interface, remotePort),
		RemoteDeviceID:   DefaultRemoteDeviceID,
		RemoteGUIAddress: net.JoinHostPort(dev.Interface, strconv.Itoa(remoteGUIPort)),
		LocalGUIPort:     guiPort,
		LocalPort:        listenPort,
		RemoteGUIPort:    remoteGUIPort,
		RemotePort:       remotePort,
		Type:             "sendonly",
		IgnoreDelete:     true,
		Verbose:          dev.Sync.Verbose,
		Folders:          []*Folder{},
		RescanInterval:   strconv.Itoa(dev.Sync.RescanInterval),
		Compression:      compression,
		timeout:          dev.Timeout.Default,
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
		return fmt.Errorf("failed to create %s: %w", s.Home, err)
	}

	if err := s.UpdateConfig(); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(s.Home, certFile), cert, 0600); err != nil {
		return fmt.Errorf("failed to write syncthing certificate: %w", err)
	}

	if err := os.WriteFile(filepath.Join(s.Home, keyFile), key, 0600); err != nil {
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

	if err := os.WriteFile(filepath.Join(s.Home, configFile), buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write syncthing configuration file: %w", err)
	}

	return nil
}

// Run starts up a local syncthing process to serve files from.
func (s *Syncthing) Run() error {
	if err := s.initConfig(); err != nil {
		return err
	}

	if s.ResetDatabase {
		cmd := exec.Command(s.binPath, "-home", s.Home, "-reset-database")
		output, err := cmd.CombinedOutput()
		if err != nil {
			oktetoLog.Errorf("error resetting syncthing database: %s\n%s", err.Error(), output)
		}
	}

	cmdArgs := []string{
		"-home", s.Home,
		"-no-browser",
		"-logfile", s.LogPath,
		"-log-max-old-files=0",
	}
	if s.Verbose {
		cmdArgs = append(cmdArgs, "-verbose")
	}

	s.cmd = exec.Command(s.binPath, cmdArgs...) // nolint: gas, gosec
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

// WaitForPing waits for syncthing to be ready
func (s *Syncthing) WaitForPing(ctx context.Context, local bool) error {
	ticker := time.NewTicker(300 * time.Millisecond)
	to := time.Now().Add(s.timeout)

	oktetoLog.Infof("waiting for syncthing local=%t to be ready", local)
	for retries := 0; ; retries++ {
		select {
		case <-ticker.C:
			if s.Ping(ctx, local) {
				oktetoLog.Infof("syncthing local=%t is ready", local)
				return nil
			}
			if retries%5 == 0 {
				oktetoLog.Infof("syncthing local=%t is not ready yet", local)
			}

			if time.Now().After(to) && retries > 10 {
				return fmt.Errorf("syncthing local=%t didn't respond after %s", local, s.timeout.String())
			}

		case <-ctx.Done():
			oktetoLog.Infof("syncthing.WaitForPing cancelled local=%t", local)
			return ctx.Err()
		}
	}
}

// Ping checks if syncthing is available
func (s *Syncthing) Ping(ctx context.Context, local bool) bool {
	_, err := s.APICall(ctx, "rest/system/ping", "GET", http.StatusOK, nil, local, nil, false, 0)
	if err == nil {
		return true
	}
	if strings.Contains(err.Error(), "Client.Timeout") {
		return true
	}
	oktetoLog.Infof("error pinging syncthing: %s", err.Error())
	return false
}

// Overwrite overwrites local changes to the remote syncthing
func (s *Syncthing) Overwrite(ctx context.Context) error {
	for _, folder := range s.Folders {
		oktetoLog.Infof("overriding local changes to the remote syncthing path=%s", folder.LocalPath)
		params := getFolderParameter(folder)
		_, err := s.APICall(ctx, "rest/db/override", "POST", http.StatusOK, params, true, nil, false, maxRetries)
		if err != nil {
			oktetoLog.Infof("error posting 'rest/db/override' syncthing API: %s", err)
			if strings.Contains(err.Error(), "Client.Timeout") {
				return oktetoErrors.ErrBusySyncthing
			}
			return oktetoErrors.ErrLostSyncthing
		}
		folder.Overwritten = true
	}
	return nil
}

// IsAllOverwritten checks if all overwrite operations has been completed
func (s *Syncthing) IsAllOverwritten() bool {
	for _, folder := range s.Folders {
		if !folder.Overwritten {
			return false
		}
	}
	return true
}

// WaitForConnected waits for local and remote syncthing to be connected
func (s *Syncthing) WaitForConnected(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	oktetoLog.Info("waiting for remote device to be connected")
	to := time.Now().Add(s.timeout)
	for retries := 0; ; retries++ {
		connections := &Connections{}
		body, err := s.APICall(ctx, "rest/system/connections", "GET", http.StatusOK, nil, true, nil, true, maxRetries)
		if err != nil {
			oktetoLog.Infof("error getting connections: %s", err.Error())
			if strings.Contains(err.Error(), "Client.Timeout") {
				return oktetoErrors.ErrBusySyncthing
			}
			return oktetoErrors.ErrLostSyncthing
		}
		err = json.Unmarshal(body, connections)
		if err != nil {
			oktetoLog.Infof("error unmarshalling connections: %s", err.Error())
			return oktetoErrors.ErrLostSyncthing
		}

		if connection, ok := connections.Connections[DefaultRemoteDeviceID]; ok {
			if connection.Connected {
				return nil
			}
		}

		if time.Now().After(to) && retries > 10 {
			oktetoLog.Infof("remote syncthing connection not completed after %s, please try again", s.timeout.String())
			return oktetoErrors.ErrLostSyncthing
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			oktetoLog.Info("call to syncthing.WaitForConnected canceled")
			return ctx.Err()
		}
	}
}

// WaitForScanning waits for syncthing to finish initial scanning
func (s *Syncthing) WaitForScanning(ctx context.Context, local bool) error {
	for _, folder := range s.Folders {
		if err := s.waitForFolderScanning(ctx, folder, local); err != nil {
			return err
		}
	}
	return nil
}

func (s *Syncthing) waitForFolderScanning(ctx context.Context, folder *Folder, local bool) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	oktetoLog.Infof("waiting for initial scan to complete path=%s local=%t", folder.LocalPath, local)

	timeoutDuration := s.timeout * 10
	to := time.Now().Add(timeoutDuration) // 5 minutes

	for retries := 0; ; retries++ {
		status, err := s.GetSyncthingStatus(ctx, folder, local)
		if err != nil && err != oktetoErrors.ErrBusySyncthing {
			return err
		}

		if status != "" {
			if retries%100 == 0 {
				// one log every 10 seconds
				oktetoLog.Infof("syncthing folder local=%t is '%s'", local, status)
			}
			if status != "scanning" && status != "scan-waiting" {
				oktetoLog.Infof("syncthing folder local=%t finished scanning: '%s'", local, status)
				return nil
			}
		}

		if time.Now().After(to) && retries > 10 {
			return fmt.Errorf("initial file scan not completed after %s, please try again", s.timeout.String())
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			oktetoLog.Info("call to syncthing.waitForFolderScanning canceled")
			return ctx.Err()
		}
	}
}

// GetCompletion returns the syncthing completion
func (s *Syncthing) GetCompletion(ctx context.Context, local bool, device string) (*Completion, error) {
	params := map[string]string{"device": device}
	completion := &Completion{}
	body, err := s.APICall(ctx, "rest/db/completion", "GET", http.StatusOK, params, local, nil, true, maxRetries)
	if err != nil {
		oktetoLog.Infof("error calling 'rest/db/completion' local=%t syncthing API: %s", local, err)
		if strings.Contains(err.Error(), "Client.Timeout") {
			return nil, oktetoErrors.ErrBusySyncthing
		}
		return nil, oktetoErrors.ErrLostSyncthing
	}
	err = json.Unmarshal(body, completion)
	if err != nil {
		oktetoLog.Infof("error unmarshalling 'rest/db/completion' local=%t syncthing API: %s", local, err)
		return nil, oktetoErrors.ErrLostSyncthing
	}
	return completion, nil
}

// IsHealthy returns the syncthing error or nil
func (s *Syncthing) IsHealthy(ctx context.Context, local bool, max int) error {
	pullErrors, err := s.GetPullErrors(ctx, local)
	if err != nil {
		if err == oktetoErrors.ErrBusySyncthing {
			return nil
		}
		return err
	}
	if pullErrors == 0 {
		isHealthyRetries = 0
		return nil
	}

	isHealthyRetries++
	err = s.GetFolderErrors(ctx, local)
	if err != nil {
		oktetoLog.Infof("syncthing error local=%t retry %d: %s", local, isHealthyRetries, err.Error())
	}
	if err == oktetoErrors.ErrInsufficientSpace {
		return err
	}

	if isHealthyRetries <= max {
		return nil
	}
	if err == nil || err == oktetoErrors.ErrBusySyncthing {
		return oktetoErrors.ErrUnknownSyncError
	}
	return err
}

// GetSyncthingStatus returns the syncthing status
func (s *Syncthing) GetSyncthingStatus(ctx context.Context, folder *Folder, local bool) (string, error) {
	params := map[string]string{
		"folder":  GetFolderName(folder),
		"since":   "0",
		"limit":   "1",
		"timeout": "0",
		"events":  "StateChanged",
	}
	scList := []StateChangedEvent{}
	body, err := s.APICall(ctx, "rest/events", "GET", http.StatusOK, params, local, nil, true, maxRetries)
	if err != nil {
		oktetoLog.Infof("error getting events: %s", err.Error())
		if strings.Contains(err.Error(), "Client.Timeout") {
			return "", oktetoErrors.ErrBusySyncthing
		}
		return "", oktetoErrors.ErrLostSyncthing
	}

	err = json.Unmarshal(body, &scList)
	if err != nil {
		oktetoLog.Infof("error unmarshalling events: %s", err.Error())
		return "", oktetoErrors.ErrLostSyncthing
	}

	if len(scList) != 0 {
		return scList[0].Data.State, nil
	}

	if v, ok := cachedStatus[local]; ok {
		return v, nil
	}

	delete(params, "events")
	delete(params, "limit")

	body, err = s.APICall(ctx, "rest/events", "GET", http.StatusOK, params, local, nil, true, maxRetries)
	if err != nil {
		oktetoLog.Infof("error getting events: %s", err.Error())
		if strings.Contains(err.Error(), "Client.Timeout") {
			return "", oktetoErrors.ErrBusySyncthing
		}
		return "", oktetoErrors.ErrLostSyncthing
	}

	err = json.Unmarshal(body, &scList)
	if err != nil {
		oktetoLog.Infof("error unmarshalling events: %s", err.Error())
		return "", oktetoErrors.ErrLostSyncthing
	}

	for i := len(scList) - 1; i >= 0; i-- {
		if scList[i].Type == "StateChanged" {
			cachedStatus[local] = scList[i].Data.State
			return cachedStatus[local], nil
		}
	}

	cachedStatus[local] = "scanning"
	return cachedStatus[local], nil

}

// GetPullErrors returns the syncthing current pull errors
func (s *Syncthing) GetPullErrors(ctx context.Context, local bool) (int64, error) {
	params := map[string]string{
		"since":   "0",
		"limit":   "1",
		"timeout": "0",
		"events":  "FolderSummary",
	}
	fsList := []FolderSummaryEvent{}
	body, err := s.APICall(ctx, "rest/events", "GET", http.StatusOK, params, local, nil, true, maxRetries)
	if err != nil {
		oktetoLog.Infof("error getting events: %s", err.Error())
		if strings.Contains(err.Error(), "Client.Timeout") {
			return 0, oktetoErrors.ErrBusySyncthing
		}
		return 0, oktetoErrors.ErrLostSyncthing
	}

	err = json.Unmarshal(body, &fsList)
	if err != nil {
		oktetoLog.Infof("error unmarshalling events: %s", err.Error())
		return 0, oktetoErrors.ErrLostSyncthing
	}

	if len(fsList) != 0 {
		return fsList[0].Data.Summary.PullErrors, nil
	}

	if v, ok := cachedPullErrors[local]; ok {
		return v, nil
	}

	delete(params, "events")
	delete(params, "limit")

	body, err = s.APICall(ctx, "rest/events", "GET", http.StatusOK, params, local, nil, true, maxRetries)
	if err != nil {
		oktetoLog.Infof("error getting events: %s", err.Error())
		if strings.Contains(err.Error(), "Client.Timeout") {
			return 0, oktetoErrors.ErrBusySyncthing
		}
		return 0, oktetoErrors.ErrLostSyncthing
	}

	err = json.Unmarshal(body, &fsList)
	if err != nil {
		oktetoLog.Infof("error unmarshalling events: %s", err.Error())
		return 0, oktetoErrors.ErrLostSyncthing
	}

	for i := len(fsList) - 1; i >= 0; i-- {
		if fsList[i].Type == "FolderSummary" {
			cachedPullErrors[local] = fsList[i].Data.Summary.PullErrors
			return cachedPullErrors[local], nil
		}
	}

	cachedPullErrors[local] = 0
	return cachedPullErrors[local], nil
}

// GetFolderErrors returns the last folder errors
func (s *Syncthing) GetFolderErrors(ctx context.Context, local bool) error {
	params := map[string]string{
		"since":   "0",
		"limit":   "1",
		"timeout": "0",
		"events":  "FolderErrors",
	}
	folderErrorsList := []FolderErrorEvent{}
	body, err := s.APICall(ctx, "rest/events", "GET", http.StatusOK, params, local, nil, true, maxRetries)
	if err != nil {
		oktetoLog.Infof("error getting events: %s", err.Error())
		if strings.Contains(err.Error(), "Client.Timeout") {
			return oktetoErrors.ErrBusySyncthing
		}
		return oktetoErrors.ErrLostSyncthing
	}

	err = json.Unmarshal(body, &folderErrorsList)
	if err != nil {
		oktetoLog.Infof("error unmarshalling events: %s", err.Error())
		return oktetoErrors.ErrLostSyncthing
	}

	folderErrors := FolderErrorEvent{
		Data: DataFolderErrorEvent{},
	}
	if len(folderErrorsList) != 0 {
		folderErrors = folderErrorsList[0]
	} else {
		if v, ok := cachedFolderErrors[local]; ok {
			folderErrors = v
		} else {
			delete(params, "events")
			delete(params, "limit")

			body, err = s.APICall(ctx, "rest/events", "GET", http.StatusOK, params, local, nil, true, maxRetries)
			if err != nil {
				oktetoLog.Infof("error getting events: %s", err.Error())
				if strings.Contains(err.Error(), "Client.Timeout") {
					return oktetoErrors.ErrBusySyncthing
				}
				return oktetoErrors.ErrLostSyncthing
			}

			err = json.Unmarshal(body, &folderErrorsList)
			if err != nil {
				oktetoLog.Infof("error unmarshalling events: %s", err.Error())
				return oktetoErrors.ErrLostSyncthing
			}

			cachedFolderErrors[local] = folderErrors
			for i := len(folderErrorsList) - 1; i >= 0; i-- {
				if folderErrorsList[i].Type == "FolderErrors" {
					cachedFolderErrors[local] = folderErrorsList[i]
					folderErrors = cachedFolderErrors[local]
				}
			}
		}
	}

	if len(folderErrors.Data.Errors) == 0 {
		return nil
	}

	errMsg := folderErrors.Data.Errors[0].Error

	if strings.Contains(errMsg, "insufficient space") {
		oktetoLog.Infof("syncthing insufficient space local=%t: %s", local, errMsg)
		return oktetoErrors.ErrInsufficientSpace
	}

	oktetoLog.Infof("syncthing pull error local=%t: %s", local, errMsg)
	return fmt.Errorf("%s: %s", folderErrors.Data.Errors[0].Path, errMsg)
}

// GetSystemErrors returns the system errors identified by syncthing
func (s *Syncthing) GetSystemErrors(ctx context.Context, local bool) (*SystemErrors, error) {
	var sysErrs *SystemErrors
	body, err := s.APICall(ctx, "rest/system/error", "GET", http.StatusOK, nil, local, nil, true, maxRetries)
	if err != nil {
		oktetoLog.Infof("error getting system errors: %s", err.Error())
		if strings.Contains(err.Error(), "Client.Timeout") {
			return nil, oktetoErrors.ErrBusySyncthing
		}
		return nil, oktetoErrors.ErrLostSyncthing
	}

	err = json.Unmarshal(body, &sysErrs)
	if err != nil {
		oktetoLog.Infof("error unmarshalling system errors: %s", err.Error())
		return nil, oktetoErrors.ErrLostSyncthing
	}

	return sysErrs, nil
}

func (s *Syncthing) IsLocalRunningOutOfSpace(ctx context.Context) bool {
	sysErrs, err := s.GetSystemErrors(ctx, true)
	if err != nil {
		oktetoLog.Infof("error getting system errors: %s", err.Error())
		return false
	}
	for _, sysErr := range sysErrs.Errors {
		if strings.Contains(sysErr.Message, "insufficient space") {
			return true
		}
	}
	return false
}

// GetInSynchronizationFile the files syncthing
func (s *Syncthing) GetInSynchronizationFile(ctx context.Context) string {
	events := []ItemEvent{}
	params := map[string]string{
		"device":  DefaultRemoteDeviceID,
		"since":   "0",
		"limit":   "1",
		"timeout": "0",
		"events":  "DownloadProgress",
	}
	body, err := s.APICall(ctx, "rest/events", "GET", http.StatusOK, params, false, nil, true, 0)
	if err != nil {
		oktetoLog.Infof("error getting GetInSynchronizationItem: %s", err.Error())
		return ""
	}

	if err := json.Unmarshal(body, &events); err != nil {
		oktetoLog.Infof("error unmarshalling events: %s", err.Error())
		return ""
	}

	if len(events) == 0 {
		return ""
	}

	return getInSynchronizationLargestFile(events[len(events)-1])
}

func getInSynchronizationLargestFile(e ItemEvent) string {
	result := ""
	var largerFileSize int64
	for _, folderStatus := range e.Data {
		for fileName := range folderStatus {
			fileSize := folderStatus[fileName].BytesTotal
			if fileSize > largerFileSize {
				result = fileName
				largerFileSize = fileSize
			}
		}
	}
	return result
}

// Restart restarts the syncthing process
func (s *Syncthing) Restart(ctx context.Context) error {
	_, err := s.APICall(ctx, "rest/system/restart", "POST", http.StatusOK, nil, true, nil, false, maxRetries)
	return err
}

// HardTerminate halts the background process, waits for 1s and kills the process if it is still running
func (s *Syncthing) HardTerminate() error {
	oktetoLog.Info("termitating previous syncthing process")
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
				oktetoLog.Infof("error getting name for process %d: %s", p.Pid, err.Error())
			}
			continue
		}

		if name == "" {
			oktetoLog.Infof("ignoring pid %d with no name: %v", p.Pid, p)
			continue
		}

		if !strings.Contains(name, "syncthing") {
			continue
		}

		cmdline, err := p.Cmdline()
		if err != nil {
			return err
		}

		oktetoLog.Infof("checking syncthing home '%s' with command '%s'", s.Home, cmdline)
		if !strings.Contains(cmdline, fmt.Sprintf("-home %s", s.Home)) {
			continue
		}

		oktetoLog.Infof("terminating syncthing %d with wait: %s", p.Pid, s.Home)
		if err := terminate(p, true); err != nil {
			oktetoLog.Infof("error terminating syncthing %d with wait: %s", p.Pid, err.Error())
		}

		oktetoLog.Infof("terminated syncthing %d with wait: %s", p.Pid, s.Home)
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
		return fmt.Errorf("error getting syncthing process %d: %w", s.pid, err)
	}
	oktetoLog.Infof("terminating syncthing %d without wait", s.pid)
	if err := terminate(p, false); err != nil {
		return fmt.Errorf("error terminating syncthing %d without wait: %w", p.Pid, err)
	}
	oktetoLog.Infof("terminated syncthing %d without wait", s.pid)
	return nil
}

// SaveConfig saves the syncthing object in the dev home folder
func (s *Syncthing) SaveConfig(dev *model.Dev) error {
	marshalled, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	syncthingInfoFile := getInfoFile(dev.Namespace, dev.Name)
	if err := os.WriteFile(syncthingInfoFile, marshalled, 0600); err != nil {
		return fmt.Errorf("failed to write syncthing info file: %w", err)
	}

	return nil
}

// Load loads the syncthing object from the dev home folder
func Load(dev *model.Dev) (*Syncthing, error) {
	syncthingInfoFile := getInfoFile(dev.Namespace, dev.Name)
	b, err := os.ReadFile(syncthingInfoFile)
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
		oktetoLog.Info("the home directory is not set when deleting")
		return nil
	}

	if _, err := filepath.Rel(config.GetOktetoHome(), s.Home); err != nil || config.GetOktetoHome() == s.Home {
		oktetoLog.Errorf("%s is not inside %s, ignoring", s.Home, config.GetOktetoHome())
		return nil
	}

	if err := os.RemoveAll(s.Home); err != nil {
		oktetoLog.Infof("failed to remote syncthing home directory at %s: %s", s.Home, err)
		return nil
	}

	parentDir := filepath.Dir(s.Home)
	if parentDir != "." {
		empty, err := isDirEmpty(parentDir)
		if err != nil {
			oktetoLog.Infof("failed to see if %s is empty: %s", parentDir, err)
			return nil
		}

		if empty {
			if err := os.RemoveAll(parentDir); err != nil {
				oktetoLog.Infof("couldn't delete folder: %s", err)
				return nil
			}

			oktetoLog.Infof("removed %s", parentDir)
		}
	}

	return nil
}

func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			oktetoLog.Debugf("Error closing file %s: %s", path, err)
		}
	}()

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
	return map[string]string{"folder": GetFolderName(folder), "device": DefaultRemoteDeviceID}
}

func GetFolderName(folder *Folder) string {
	return fmt.Sprintf("okteto-%s", folder.Name)
}

func getInfoFile(namespace, name string) string {
	return filepath.Join(config.GetAppHome(namespace, name), "syncthing.info")
}

// GetLogFile returns the path to the syncthing log file
func GetLogFile(namespace, name string) string {
	return filepath.Join(config.GetAppHome(namespace, name), "syncthing.log")
}
