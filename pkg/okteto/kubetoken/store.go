package kubetoken

import (
	"fmt"
	"os"
	"path/filepath"
)

func NewFileByteStore(fileName string) *FileByteStore {
	return &FileByteStore{
		FileName: fileName,
		osStat:   os.Stat,
		createFile: func(filename string) (*os.File, error) {
			fp := filepath.Dir(filename)
			folder := filepath.Base(filename)
			if err := os.MkdirAll(fp, 0764); err != nil {
				return nil, fmt.Errorf("error creating folder %q for %q: %w", folder, filename, err)
			}
			return os.Create(filename)
		},
		osReadFile: os.ReadFile,
		writeFile: func(filename string, data []byte) error {
			return os.WriteFile(filename, data, 0764)
		},
	}
}

type FileByteStore struct {
	FileName   string
	osStat     func(name string) (os.FileInfo, error)
	createFile func(name string) (*os.File, error)
	osReadFile func(filename string) ([]byte, error)
	writeFile  func(filename string, data []byte) error
}

func (s *FileByteStore) Get() ([]byte, error) {
	if _, err := s.osStat(s.FileName); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error checking if file exists: %w", err)
		}

		f, err := s.createFile(s.FileName)
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
