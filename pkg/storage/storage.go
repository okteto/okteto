package storage

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/okteto/cnd/pkg/config"
	"github.com/okteto/cnd/pkg/model"
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

//Insert inserts a new service entry
func Insert(
	ctx context.Context, wg *sync.WaitGroup,
	namespace string, dev *model.Dev, host string) error {

	if err := insert(namespace, dev, host); err != nil {
		return err
	}

	wg.Add(1)

	go func() {
		defer wg.Done()

		<-ctx.Done()
		if err := Stop(namespace, dev); err != nil {
			log.Error(err)
		}
		log.Debug("insert clean shutdown")
		return
	}()

	return nil
}

func insert(namespace string, dev *model.Dev, host string) error {
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
	s, err := load()
	if err != nil {
		return err
	}

	fullName := getFullName(namespace, dev)
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
