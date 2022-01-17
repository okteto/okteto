// Copyright 2021 The Okteto Authors
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

package dev

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/constants"
)

//TranslateDeprecatedVolumeFields translates workdir and volumes into sync
func (dev *Dev) TranslateDeprecatedVolumeFields() error {
	if dev.Workdir == "" && len(dev.Sync.Folders) == 0 {
		dev.Workdir = "/okteto"
	}
	if err := dev.translateDeprecatedWorkdir(nil); err != nil {
		return err
	}
	dev.translateDeprecatedVolumes()

	for _, s := range dev.Services {
		if err := s.translateDeprecatedWorkdir(dev); err != nil {
			return err
		}
		s.translateDeprecatedVolumes()
	}
	return nil
}

func (dev *Dev) translateDeprecatedWorkdir(main *Dev) error {
	if dev.Workdir == "" || len(dev.Sync.Folders) > 0 {
		return nil
	}
	if main != nil {
		return fmt.Errorf("'workdir' is not supported to define your synchronized folders in 'services'. Use the field 'sync' instead (%s)", constants.SyncFieldDocsURL)
	}
	dev.Sync.Folders = append(
		dev.Sync.Folders,
		SyncFolder{
			LocalPath:  ".",
			RemotePath: dev.Workdir,
		},
	)
	return nil
}

func (dev *Dev) translateDeprecatedVolumes() {
	volumes := []Volume{}
	for _, v := range dev.Volumes {
		if v.LocalPath == "" {
			volumes = append(volumes, v)
			continue
		}
		dev.Sync.Folders = append(dev.Sync.Folders, SyncFolder(v))
	}
	dev.Volumes = volumes
}

//IsSubPathFolder checks if a sync folder is a subpath of another sync folder
func (dev *Dev) IsSubPathFolder(path string) (bool, error) {
	found := false
	for _, sync := range dev.Sync.Folders {
		rel, err := filepath.Rel(sync.LocalPath, path)
		if err != nil {
			log.Infof("error making rel '%s' and '%s'", sync.LocalPath, path)
			return false, err
		}
		if strings.HasPrefix(rel, "..") {
			continue
		}
		found = true
		if rel != "." {
			return true, nil
		}
	}
	if found {
		return false, nil
	}
	return false, errors.ErrNotFound
}

//ComputeParentSyncFolder gets the parent local path for sync folders
func (dev *Dev) ComputeParentSyncFolder() {
	pathSplits := map[int]string{}
	maxIndex := -1
	for i, sync := range dev.Sync.Folders {
		path := filepath.ToSlash(sync.LocalPath)
		if i == 0 {
			for j, subPath := range strings.Split(path, "/") {
				pathSplits[j] = subPath
				maxIndex = j
			}
			continue
		}
		for j, subPath := range strings.Split(path, "/") {
			if j > maxIndex {
				break
			}
			oldSubPath, ok := pathSplits[j]
			if !ok || oldSubPath != subPath {
				maxIndex = j - 1
				break
			}
		}
	}
	dev.parentSyncFolder = "/"
	for i := 1; i <= maxIndex; i++ {
		dev.parentSyncFolder = filepath.ToSlash(filepath.Join(dev.parentSyncFolder, pathSplits[i]))
	}

	dev.parentSyncFolder = dev.parentSyncFolder[len(filepath.VolumeName(dev.parentSyncFolder)):]

}

func getDataSubPath(path string) string {
	return filepath.ToSlash(filepath.Join(constants.DataSubPath, path))
}

func (dev *Dev) getSourceSubPath(path string) string {
	path = path[len(filepath.VolumeName(path)):]
	rel, err := filepath.Rel(dev.parentSyncFolder, filepath.ToSlash(path))
	if err != nil {
		log.Fatalf("error on getSourceSubPath of '%s': %s", path, err.Error())
	}
	return filepath.ToSlash(filepath.Join(constants.SourceCodeSubPath, filepath.ToSlash(rel)))
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
		return constants.OktetoDefaultPVSize
	}
	if dev.PersistentVolumeInfo.Size == "" {
		return constants.OktetoDefaultPVSize
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

//AreDefaultPersistentVolumeValues check if pv is default
func (dev *Dev) AreDefaultPersistentVolumeValues() bool {
	if dev.PersistentVolumeInfo != nil {
		if dev.PersistentVolumeSize() == constants.OktetoDefaultPVSize && dev.PersistentVolumeStorageClass() == "" && dev.PersistentVolumeEnabled() {
			return true
		}
	}
	return false

}
