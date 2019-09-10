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
	"sync"
	"text/template"
	"time"

	"crypto/tls"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/syncthing/syncthing/lib/syncthing"
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
	app              *syncthing.App
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

	s := &Syncthing{
		APIKey:           "cnd",
		Client:           NewAPIClient(),
		Dev:              dev,
		DevPath:          dev.DevPath,
		FileWatcherDelay: DefaultFileWatcherDelay,
		GUIAddress:       fmt.Sprintf("127.0.0.1:%d", guiPort),
		Home:             filepath.Join(config.GetHome(), dev.Namespace, dev.Name),
		LogPath:          filepath.Join(config.GetHome(), dev.Namespace, dev.Name, logFile),
		ListenAddress:    fmt.Sprintf("127.0.0.1:%d", listenPort),
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

// UpdateConfig updates the syncthing config file
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
	if err := s.initSyncthingApp(); err != nil {
		return err
	}

	s.app.Start()
	log.Infof("syncthing running on http://%s and tcp://%s", s.GUIAddress, s.ListenAddress)

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		s.app.Stop(syncthing.ExitSuccess)
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
func (s *Syncthing) WaitForCompletion(ctx context.Context, wg *sync.WaitGroup, dev *model.Dev) error {
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
		log.Infof("syncthing folder is %.2f%%, needBytes %d, needDeletes %d",
			(float64(completion.GlobalBytes-completion.NeedBytes)/float64(completion.GlobalBytes))*100,
			completion.NeedBytes,
			completion.NeedDeletes,
		)
		if completion.NeedBytes == 0 {
			return nil
		}
	}
}

// Restart restarts the syncthing process
func (s *Syncthing) Restart(ctx context.Context, wg *sync.WaitGroup) error {
	log.Infof("restarting syncthing...")
	status := s.app.Stop(syncthing.ExitSuccess)
	if status == syncthing.ExitError {
		log.Infof("failed to restart syncthing")
		return fmt.Errorf("internal server error")
	}

	s.app.Wait()
	log.Infof("syncthing stopped")
	if err := s.initSyncthingApp(); err != nil {
		return err
	}

	s.app.Start()
	log.Infof("restarted syncthing...")
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

func (s *Syncthing) initSyncthingApp() error {
	if err := s.initConfig(); err != nil {
		return err
	}

	cfgFile := filepath.Join(s.Home, configFile)
	dbFile := filepath.Join(s.Home, "index-v0.14.0.db")
	c, err := tls.X509KeyPair(cert, key)
	if err != nil {
		log.Errorf("failed to generate certs for synchronization service: %s", err)
		return fmt.Errorf("internal server error")
	}

	cfg, err := syncthing.LoadConfigAtStartup(cfgFile, c, false, true)
	if err != nil {
		log.Errorf("failed to start synchronization service: %s", err)
		return fmt.Errorf("internal server error")
	}

	ldb, err := syncthing.OpenGoleveldb(dbFile)
	if err != nil {
		log.Errorf("failed to load the DB for the synchronization service: %s", err)
		return fmt.Errorf("internal server error")
	}

	s.app = syncthing.New(cfg, ldb, c, syncthing.Options{
		NoUpgrade: true,
		Verbose:   false,
	})

	return nil
}
