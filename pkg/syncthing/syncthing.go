package syncthing

import (
	"bytes"
	"context"
	"encoding/json"
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
	"time"

	"github.com/okteto/cnd/pkg/config"
	"github.com/okteto/cnd/pkg/model"
	log "github.com/sirupsen/logrus"
)

var (
	configTemplate = template.Must(template.New("syncthingConfig").Parse(configXML))
)

const (
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
	RestClient       *http.Client
}

// NewSyncthing constructs a new Syncthing.
func NewSyncthing(namespace, deployment string, devList []*model.Dev) (*Syncthing, error) {

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
		home:             path.Join(config.GetCNDHome(), namespace, deployment),
		Name:             deployment,
		DevList:          devList,
		Namespace:        namespace,
		RemoteAddress:    fmt.Sprintf("tcp://localhost:%d", remotePort),
		RemoteDeviceID:   DefaultRemoteDeviceID,
		FileWatcherDelay: DefaultFileWatcherDelay,
		GUIAddress:       fmt.Sprintf("127.0.0.1:%d", guiPort),
		ListenAddress:    fmt.Sprintf("0.0.0.0:%d", listenPort),
		RestClient:       NewRestClient(),
	}

	return s, nil
}

//NewRestClient returns a new rest client configured to call the syncthing api
func NewRestClient() *http.Client {
	return &http.Client{
		Timeout:   15 * time.Second,
		Transport: &addAPIKeyTransport{http.DefaultTransport},
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
	defer wg.Done()

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

	log.Infof("Syncthing running on http://%s and tcp://%s", s.GUIAddress, s.ListenAddress)

	go func() {
		<-ctx.Done()
		if err := s.Stop(); err != nil {
			log.Error(err)
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

type addAPIKeyTransport struct {
	T http.RoundTripper
}

type syncthingConnections struct {
	Connections map[string]struct {
		Connected bool `json:"connected,omitempty"`
	} `json:"connections,omitempty"`
}

//Monitor verifies that syncthing is not in a disconnected state. If so, it sends a message to the
// disconnected channel and exits
func (s *Syncthing) Monitor(ctx context.Context, disconnected chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			if !s.isConnectedToRemote() {
				disconnected <- struct{}{}
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func (s *Syncthing) isConnectedToRemote() bool {
	body, err := s.GetFromAPI("rest/system/connections")
	if err != nil {
		log.Debugf("error when getting connections from the api: %s", err)
		return true
	}

	var conns syncthingConnections
	if err := json.Unmarshal(body, &conns); err != nil {
		return true
	}

	if val, ok := conns.Connections[s.RemoteDeviceID]; ok {
		return val.Connected
	}

	log.Infof("RemoteDeviceID %s missing from the response", s.RemoteDeviceID)
	return true
}

// GetFromAPI calls the syncthing API and returns the parsed json or an error
func (s *Syncthing) GetFromAPI(url string) ([]byte, error) {
	urlPath := path.Join(s.GUIAddress, url)
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", urlPath), nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("limit", "30")
	req.URL.RawQuery = q.Encode()

	resp, err := s.RestClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad response from syncthing api %s %d: %s", req.URL.String(), resp.StatusCode, string(body))
	}

	return body, nil
}

func (akt *addAPIKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("X-API-Key", "cnd")
	return akt.T.RoundTrip(req)
}
