// Copyright 2023 The Okteto Authors
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

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

const (
	cloudDefaultVolumeSize = "2Gi"
	defaultVolumeSize      = "5Gi"
)

func (dev *Dev) translateDeprecatedVolumeFields() error {
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
		return fmt.Errorf("'workdir' is not supported to define your synchronized folders in 'services'. Use the field 'sync' instead (%s)", syncFieldDocsURL)
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

// IsSubPathFolder checks if a sync folder is a subpath of another sync folder
func (dev *Dev) IsSubPathFolder(path string) (bool, error) {
	found := false
	for _, sync := range dev.Sync.Folders {
		rel, err := filepath.Rel(sync.LocalPath, path)
		if err != nil {
			oktetoLog.Infof("error making rel '%s' and '%s'", sync.LocalPath, path)
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
	return false, oktetoErrors.ErrNotFound
}

func (dev *Dev) computeParentSyncFolder() {
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
	return filepath.ToSlash(filepath.Join(DataSubPath, path))
}

func (dev *Dev) getSourceSubPath(path string) string {
	sourceSubPath := path[len(filepath.VolumeName(path)):]
	if dev.parentSyncFolder == "" {
		dev.parentSyncFolder = "."
	}
	rel, err := filepath.Rel(dev.parentSyncFolder, filepath.ToSlash(sourceSubPath))
	if filepath.IsAbs(sourceSubPath) {
		if err != nil || strings.HasPrefix(rel, "..") {
			if err != nil {
				oktetoLog.Debugf("error on getSourceSubPath of '%s': %s", path, err.Error())
			}
			if filepath.IsAbs(path) {
				oktetoLog.Info("could not retrieve subpath")
			} else {
				p, err := filepath.Abs(path)
				if err != nil {
					oktetoLog.Debugf("error on getSourceSubPath of '%s': %s", path, err.Error())
				}
				return filepath.ToSlash(p)
			}
		}
	}
	return filepath.ToSlash(filepath.Join(SourceCodeSubPath, filepath.ToSlash(rel)))
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
		return dev.getDefaultPersistentVolumeSize()
	}
	if dev.PersistentVolumeInfo.Size == "" {
		return dev.getDefaultPersistentVolumeSize()
	}
	return dev.PersistentVolumeInfo.Size
}

func (dev *Dev) isOktetoCloud() bool { // TODO: inject this
	switch dev.Context {
	case "https://cloud.okteto.com", "https://staging.okteto.dev":
		return true
	default:
		return false
	}
}

func (dev *Dev) getDefaultPersistentVolumeSize() string {
	switch {
	case dev.isOktetoCloud():
		return cloudDefaultVolumeSize
	default:
		return defaultVolumeSize
	}
}

func (dev *Dev) HasDefaultPersistentVolumeSize() bool {
	return dev.PersistentVolumeSize() == dev.getDefaultPersistentVolumeSize()
}

// PersistentVolumeStorageClass returns the persistent volume storage class
func (dev *Dev) PersistentVolumeStorageClass() string {
	if dev.PersistentVolumeInfo == nil {
		return ""
	}
	return dev.PersistentVolumeInfo.StorageClass
}

func (dev *Dev) AreDefaultPersistentVolumeValues() bool {
	if dev.PersistentVolumeInfo != nil {
		if dev.HasDefaultPersistentVolumeSize() && dev.PersistentVolumeStorageClass() == "" && dev.PersistentVolumeEnabled() {
			return true
		}
	}
	return false

}

func (dev *Dev) validatePersistentVolume() error {
	if dev.PersistentVolumeEnabled() {
		return nil
	}
	if len(dev.Services) > 0 {
		return fmt.Errorf("'persistentVolume.enabled' must be set to true to work with services")
	}
	if len(dev.Volumes) > 0 {
		return fmt.Errorf("'persistentVolume.enabled' must be set to true to use volumes")
	}
	for _, sync := range dev.Sync.Folders {
		result, err := dev.IsSubPathFolder(sync.LocalPath)
		if err != nil {
			return err
		}
		if result {
			return fmt.Errorf("'persistentVolume.enabled' must be set to true to use subfolders in the 'sync' field")
		}
	}
	return nil
}

func (dev *Dev) validateRemotePaths() error {
	for _, v := range dev.Volumes {
		if !strings.HasPrefix(v.RemotePath, "/") {
			return fmt.Errorf("relative remote paths are not supported in the field 'volumes'")
		}
		if v.RemotePath == "/" {
			return fmt.Errorf("remote path '/' is not supported in the field 'volumes'")
		}
	}
	for _, sync := range dev.Sync.Folders {
		if !strings.HasPrefix(sync.RemotePath, "/") {
			return fmt.Errorf("relative remote paths are not supported in the field 'sync'")
		}
		if sync.RemotePath == "/" {
			return fmt.Errorf("remote path '/' is not supported in the field 'sync'")
		}
	}
	return nil
}

func (dev *Dev) validateDuplicatedVolumes() error {
	seen := map[string]bool{}
	for _, v := range dev.Volumes {
		if seen[v.RemotePath] {
			return fmt.Errorf("duplicated volume '%s'", v.RemotePath)
		}
		seen[v.RemotePath] = true
	}
	return nil
}

func (dev *Dev) validateDuplicatedSyncFolders() error {
	seen := map[string]bool{}
	seenRootLocalPath := map[string]bool{}
	for _, sync := range dev.Sync.Folders {
		key := sync.LocalPath + ":" + sync.RemotePath
		if seen[key] {
			return fmt.Errorf("duplicated sync '%s'", sync)
		}
		seen[key] = true
		result, err := dev.IsSubPathFolder(sync.LocalPath)
		if err != nil {
			return err
		}
		if result {
			continue
		}
		if seenRootLocalPath[sync.LocalPath] {
			return fmt.Errorf("duplicated sync localPath '%s'", sync.LocalPath)
		}
		seenRootLocalPath[sync.LocalPath] = true
	}
	return nil
}

func (dev *Dev) validateServiceSyncFolders(main *Dev) error {
	for _, sync := range dev.Sync.Folders {
		_, err := main.IsSubPathFolder(sync.LocalPath)
		if err != nil {
			if err == oktetoErrors.ErrNotFound {
				return fmt.Errorf("LocalPath '%s' in 'services' not defined in the field 'sync' of the main development container", sync.LocalPath)
			}
			return err
		}
	}
	return nil
}

func (dev *Dev) validateVolumes(main *Dev) error {
	if len(dev.Sync.Folders) == 0 {
		return fmt.Errorf("the 'sync' field is mandatory. More info at %s", syncFieldDocsURL)
	}

	if err := dev.validateRemotePaths(); err != nil {
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

	return dev.validateServiceSyncFolders(main)
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
