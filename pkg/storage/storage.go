package storage

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cloudnativedevelopment/cnd/pkg/config"
	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/model"
	"github.com/cloudnativedevelopment/cnd/pkg/syncthing"
	ps "github.com/mitchellh/go-ps"
	yaml "gopkg.in/yaml.v2"
)

const (
	version = "1.0"
)

var (
	// ErrAlreadyRunning indicates a "cnd up" command is already running
	ErrAlreadyRunning = fmt.Errorf("up-already-running")
)

//Storage represents the cli state
type Storage struct {
	path     string
	Version  string             `yaml:"version,omitempty"`
	Services map[string]Service `yaml:"services,omitempty"`
}

//Service represents the information about a cnd service
type Service struct {
	Folder    string `yaml:"folder,omitempty"`
	Syncthing string `yaml:"syncthing,omitempty"`
	PID       int    `yaml:"pid,omitempty"`
	Pod       string `yaml:"pod,omitempty"`
}

func getSTPath() string {
	return path.Join(config.GetCNDHome(), ".state")
}

func load() (*Storage, error) {
	var s Storage
	s.path = getSTPath()
	s.Version = version
	s.Services = map[string]Service{}
	if _, err := os.Stat(getSTPath()); os.IsNotExist(err) {
		return &s, nil
	}
	bytes, err := ioutil.ReadFile(getSTPath())
	if err != nil {
		return nil, fmt.Errorf("error reading the storage file: %s", err.Error())
	}
	err = yaml.Unmarshal(bytes, &s)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling the storage file: %s", err.Error())
	}
	return &s, nil
}

//Insert inserts a new service entry, and cleans it up when the context is cancelled
func Insert(
	ctx context.Context, wg *sync.WaitGroup,
	namespace string, dev *model.Dev, host, pod string) error {

	if err := insert(namespace, dev, host, pod); err != nil {
		return err
	}

	go func() {
		wg.Add(1)
		defer wg.Done()

		<-ctx.Done()
		if err := Stop(namespace, dev); err != nil {
			log.Info(err)
		}
		log.Debug("insert clean shutdown")
		return
	}()

	return nil
}

func insert(namespace string, dev *model.Dev, host, pod string) error {
	s, err := load()
	if err != nil {
		return err
	}

	fullName := getFullName(namespace, dev)
	svc, err := newService(dev.Mount.Source, host)
	if err != nil {
		return err
	}

	if svc2, ok := s.Services[fullName]; ok {
		if svc2 == svc {
			return nil
		}

		if svc2.Syncthing != "" {
			log.Debugf("There's a service already running: %+v", svc2)
			return ErrAlreadyRunning
		}
	}

	svc.PID = os.Getpid()
	svc.Pod = pod
	s.Services[fullName] = svc
	if err := s.save(); err != nil {
		return err
	}
	return nil
}

//Get gets a service entry
func Get(namespace string, dev *model.Dev) (*Service, error) {
	s, err := load()
	if err != nil {
		return nil, err
	}

	fullName := getFullName(namespace, dev)
	svc, ok := s.Services[fullName]
	if !ok {
		return nil, fmt.Errorf("there aren't any active cloud native development environments available for '%s'", fullName)
	}
	return &svc, nil
}

//Stop marks a service entry as stopped
func Stop(namespace string, dev *model.Dev) error {
	s, err := load()
	if err != nil {
		return err
	}

	fullName := getFullName(namespace, dev)
	svc, ok := s.Services[fullName]
	if ok {
		svc.Syncthing = ""
		s.Services[fullName] = svc
		return s.save()
	}
	return nil
}

//Delete deletes a service entry
func Delete(namespace string, dev *model.Dev) error {
	fullName := getFullName(namespace, dev)
	return deleteEntry(fullName)
}

func deleteEntry(fullName string) error {
	s, err := load()
	if err != nil {
		return err
	}
	serviceFolder, err := getServiceFolder(fullName)

	sy := syncthing.Syncthing{
		Home: serviceFolder,
	}

	if err := sy.RemoveFolder(); err != nil {
		log.Infof("couldn't delete %s. Please delete manually.", serviceFolder)
	}

	delete(s.Services, fullName)

	return s.save()
}

//All returns the active cnd services
func All() map[string]Service {
	s, err := load()
	if err != nil {
		return nil
	}

	return s.Services
}

func (s *Storage) save() error {

	bytes, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("error marshalling storage: %s", err.Error())
	}
	err = ioutil.WriteFile(s.path, bytes, 0644)
	if err != nil {
		return fmt.Errorf("error writing storage: %s", err.Error())
	}
	return nil
}

func fixPath(originalPath string) (string, error) {
	if filepath.IsAbs(originalPath) {
		return originalPath, nil
	}
	folder, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return path.Join(folder, originalPath), nil
}

func newService(folder, host string) (Service, error) {
	absFolder, err := fixPath(folder)
	if err != nil {
		return Service{}, err
	}
	return Service{Folder: absFolder, Syncthing: host}, nil
}

func getFullName(namespace string, dev *model.Dev) string {
	return fmt.Sprintf("%s/%s/%s", namespace, dev.Swap.Deployment.Name, dev.Swap.Deployment.Container)
}

func getServiceFolder(fullName string) (string, error) {
	parts := strings.Split(fullName, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid state file, please remove %s from %s manualy and try again", fullName, getSTPath())
	}

	return path.Join(config.GetCNDHome(), parts[0], parts[1]), nil
}

// RemoveIfStale removes the entry from the state file if it's stale
// It will return true if the service entry was stale and it was successfully removed.
func RemoveIfStale(svc *Service, fullName string) bool {
	serviceFolder, err := getServiceFolder(fullName)
	if err != nil {
		log.Infof("state is malformed, manual action might be required: %s", err)
		return false
	}

	if _, err := os.Stat(serviceFolder); os.IsNotExist(err) {
		log.Debugf("%s doesn't exist, removing %s from state", serviceFolder, fullName)
		deleteEntry(fullName)
		return true
	}

	if svc.Syncthing == "" {
		// If syncthing value is empty, it was correctly shutdown
		return false
	}

	if !syncthing.Exists(serviceFolder) {
		log.Debugf("%s/syncthing.pid is not running anymore, removing %s from state", serviceFolder, fullName)
		deleteEntry(fullName)
		return true
	}

	if svc.PID == 0 {
		log.Debugf("%s didn't have a PID", fullName)
		return false
	}

	process, err := ps.FindProcess(svc.PID)
	if (process == nil && err == nil) || (process.Executable() != config.GetBinaryName()) {
		log.Debugf("original pid-%d is not running anymore, removing %s from state", svc.PID, fullName)
		sy := syncthing.Syncthing{
			Home: serviceFolder,
		}

		if err := sy.Stop(); err != nil {
			log.Debugf("Couldn't stop rogue syncthing at %s/syncthing.pid: %s", serviceFolder, err)
		}

		deleteEntry(fullName)
		return true
	}

	return false
}
