package storage

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

const (
	version = "1.0"
)

var (
	stPath string
)

//Storage represents the cli state
type Storage struct {
	path     string
	Version  string             `yaml:"version,omitempty"`
	Services map[string]Service `yaml:"services,omitempty"`
}

//Service represents the information about a dev mode service
type Service struct {
	Folder    string `yaml:"folder,omitempty"`
	Syncthing string `yaml:"syncthing,omitempty"`
	Container string `yaml:"container,omitempty"`
}

func init() {
	stPath = path.Join(os.Getenv("HOME"), ".cnd", ".state")
}
func load() (*Storage, error) {
	var s Storage
	s.path = stPath
	s.Version = version
	s.Services = map[string]Service{}
	if _, err := os.Stat(stPath); os.IsNotExist(err) {
		return &s, nil
	}
	bytes, err := ioutil.ReadFile(stPath)
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
func Insert(namespace, deployment, container, folder, host string) error {
	s, err := load()
	if err != nil {
		return err
	}

	fullName := fmt.Sprintf("%s/%s", namespace, deployment)
	absFolder, err := fixPath(folder)
	if err != nil {
		return err
	}
	svc := Service{Folder: absFolder, Syncthing: host, Container: container}
	if svc2, ok := s.Services[fullName]; ok {
		if svc2 == svc {
			return nil
		}
		return fmt.Errorf("there is already an entry for '%s'.\nAre you running 'cnd up %s' somewhere else?", fullName, fullName)
	}
	s.Services[fullName] = svc
	return s.save()
}

//Get gets a service entry
func Get(namespace, deployment string) (*Service, error) {
	s, err := load()
	if err != nil {
		return nil, err
	}

	fullName := fmt.Sprintf("%s/%s", namespace, deployment)
	svc, ok := s.Services[fullName]
	if !ok {
		return nil, fmt.Errorf("there is no active dev mode for '%s'", fullName)
	}
	return &svc, nil
}

//Delete deletes a service entry
func Delete(namespace, deployment string) error {
	s, err := load()
	if err != nil {
		return err
	}

	fullName := fmt.Sprintf("%s/%s", namespace, deployment)
	delete(s.Services, fullName)
	return s.save()
}

//All returns the active dev mode services
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
