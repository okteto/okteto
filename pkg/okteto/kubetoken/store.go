package kubetoken

import (
	"errors"
	"fmt"
	"os"
)

type FileByteStore struct {
	FileName string
}

func (s *FileByteStore) Get() ([]byte, error) {
	if _, err := os.Stat(s.FileName); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("error checking if file exists: %w", err)
		}

		f, err := os.Create(s.FileName)
		if err != nil {
			return nil, fmt.Errorf("error creating file: %w", err)
		}
		defer f.Close()
	}

	contents, err := os.ReadFile(s.FileName)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return contents, nil
}

func (s *FileByteStore) Set(value []byte) error {
	return os.WriteFile(s.FileName, value, 0600)
}
