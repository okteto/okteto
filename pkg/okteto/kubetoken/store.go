package kubetoken

import (
	"errors"
	"fmt"
	"os"
)

func NewFileByteStore(fileName string) *FileByteStore {
	return &FileByteStore{
		FileName:   fileName,
		osStat:     os.Stat,
		osCreate:   os.Create,
		osReadFile: os.ReadFile,
		writeFile: func(filename string, data []byte) error {
			return os.WriteFile(filename, data, 0600)
		},
	}
}

type FileByteStore struct {
	FileName   string
	osStat     func(name string) (os.FileInfo, error)
	osCreate   func(name string) (*os.File, error)
	osReadFile func(filename string) ([]byte, error)
	writeFile  func(filename string, data []byte) error
}

func (s *FileByteStore) Get() ([]byte, error) {
	if _, err := s.osStat(s.FileName); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("error checking if file exists: %w", err)
		}

		f, err := s.osCreate(s.FileName)
		if err != nil {
			return nil, fmt.Errorf("error creating file: %w", err)
		}
		defer f.Close()
	}

	contents, err := s.osReadFile(s.FileName)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return contents, nil
}

func (s *FileByteStore) Set(value []byte) error {
	return s.writeFile(s.FileName, value)
}
