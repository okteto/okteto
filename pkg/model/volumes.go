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

package model

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/okteto/okteto/pkg/errors"
)

// HasLocalVolumes returns true if the manifest has local volumes
func (dev *Dev) HasLocalVolumes() bool {
	for _, v := range dev.Volumes {
		if v.LocalPath != "" {
			return true
		}
	}
	return false
}

// HasRemoteVolumes returns true if the manifest has remote volumes
func (dev *Dev) HasRemoteVolumes() bool {
	for _, v := range dev.Volumes {
		if v.LocalPath == "" {
			return true
		}
	}
	return false
}

// IsSyncFolder returns if path must be synched
func (dev *Dev) IsSyncFolder(path string) (bool, error) {
	found := false
	for _, v := range dev.Volumes {
		if v.LocalPath == "" {
			continue
		}
		rel, err := filepath.Rel(v.LocalPath, path)
		if err != nil {
			return false, err
		}
		if strings.HasPrefix(rel, "..") {
			continue
		}
		found = true
		if rel != "." {
			return false, nil
		}
	}
	if found {
		return true, nil
	}
	return false, errors.ErrNotFound
}

// PersistentVolumeEnabled returns true if persistent volumes are enabled for dev
func (dev *Dev) PersistentVolumeEnabled() bool {
	if dev.PersistentVolumeInfo == nil {
		return true
	}
	return dev.PersistentVolumeInfo.Enabled
}

// PersistentVolumeSize returns the persistent volume size
func (dev *Dev) PersistentVolumeSize() string {
	if dev.PersistentVolumeInfo == nil {
		return OktetoDefaultPVSize
	}
	if dev.PersistentVolumeInfo.Size == "" {
		return OktetoDefaultPVSize
	}
	return dev.PersistentVolumeInfo.Size
}

// PersistentVolumeStorageClass returns the persistent volume storage class
func (dev *Dev) PersistentVolumeStorageClass() string {
	if dev.PersistentVolumeInfo == nil {
		return ""
	}
	return dev.PersistentVolumeInfo.StorageClass
}

func (dev *Dev) validatePersistentVolume() error {
	if dev.PersistentVolumeEnabled() {
		return nil
	}
	if len(dev.Services) > 0 {
		return fmt.Errorf("'persistentVolume.enabled' must be set to true to work with services")
	}
	if dev.HasRemoteVolumes() {
		return fmt.Errorf("'persistentVolume.enabled' must be set to true to use remote volumes")
	}
	for _, v := range dev.Volumes {
		result, err := dev.IsSyncFolder(v.LocalPath)
		if err != nil {
			return err
		}
		if !result {
			return fmt.Errorf("'persistentVolume.enabled' must be set to true to use subfolders in the 'volumes' field")
		}
	}
	return nil
}

func (dev *Dev) validateVolumeRemotePaths() error {
	for _, v := range dev.Volumes {
		if !strings.HasPrefix(v.RemotePath, "/") {
			return fmt.Errorf("relative remote paths are not supported as volumes")
		}
		if v.RemotePath == "/" {
			return fmt.Errorf("remote path '/' is not supported as volumes")
		}
	}
	return nil
}

func (dev *Dev) validateDuplicatedVolumes() error {
	seen := map[string]bool{}
	for _, v := range dev.Volumes {
		key := v.LocalPath + ":" + v.RemotePath
		if seen[key] {
			if v.LocalPath == "" {
				return fmt.Errorf("duplicated volume '%s'", v.RemotePath)
			}
			return fmt.Errorf("duplicated volume '%s:%s'", v.LocalPath, v.RemotePath)
		}
		seen[key] = true
	}
	return nil
}

func (dev *Dev) validateDuplicatedSyncFolders() error {
	seen := map[string]bool{}
	for _, v := range dev.Volumes {
		if v.LocalPath == "" {
			continue
		}
		result, err := dev.IsSyncFolder(v.LocalPath)
		if err != nil {
			return err
		}
		if !result {
			continue
		}
		if seen[v.LocalPath] {
			return fmt.Errorf("duplicated local volume '%s'", v.LocalPath)
		}
		seen[v.LocalPath] = true
	}
	return nil
}

func (dev *Dev) validateServiceSyncFolders(main *Dev) error {
	for _, v := range dev.Volumes {
		if v.LocalPath == "" {
			continue
		}
		_, err := main.IsSyncFolder(v.LocalPath)
		if err != nil {
			return fmt.Errorf("Local volume '%s' in '%s' not defined in the main development container", v.LocalPath, dev.Name)
		}
	}
	return nil
}

func (dev *Dev) validateVolumes(main *Dev) error {
	if !dev.HasLocalVolumes() {
		return fmt.Errorf("the 'volumes' field must define a local folder to be synched. More info at https://okteto.com/docs/reference/manifest#volumes-string-optional")
	}

	if err := dev.validateVolumeRemotePaths(); err != nil {
		return err
	}

	if err := dev.validateDuplicatedVolumes(); err != nil {
		return err
	}

	if err := dev.validateDuplicatedSyncFolders(); err != nil {
		return err
	}

	if main == nil {
		return nil
	}

	if err := dev.validateServiceSyncFolders(main); err != nil {
		return err
	}

	return nil
}

func (dev *Dev) validateExternalVolumes() error {
	for _, v := range dev.ExternalVolumes {
		if !strings.HasPrefix(v.MountPath, "/") {
			return fmt.Errorf("external volume '%s' mount path must be absolute", v.Name)
		}
		if v.MountPath == "/" {
			return fmt.Errorf("external volume '%s' mount path '/' is not supported", v.Name)
		}
	}
	return nil
}
