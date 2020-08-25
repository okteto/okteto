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
	"github.com/okteto/okteto/pkg/log"
)

func (dev *Dev) translateDeprecatedVolumeFields() error {
	if err := dev.translateDeprecatedMountPath(nil); err != nil {
		return err
	}
	if err := dev.translateDeprecatedWorkdir(nil); err != nil {
		return err
	}
	dev.translateDeprecatedVolumes()

	for _, s := range dev.Services {
		if err := s.translateDeprecatedMountPath(dev); err != nil {
			return err
		}
		if err := s.translateDeprecatedWorkdir(dev); err != nil {
			return err
		}
		s.translateDeprecatedVolumes()
	}
	return nil
}

func (dev *Dev) translateDeprecatedMountPath(main *Dev) error {
	if dev.MountPath == "" {
		return nil
	}
	if main != nil && main.MountPath == "" {
		return fmt.Errorf("'mountpath' is not supported to define your synchronized folders in 'services'. Use the field 'sync' instead (%s)", syncFieldDocsURL)
	}

	warnMessage := func() {
		log.Yellow("'mounthpath' is deprecated to define your synchronized folders. Use the field 'sync' instead (%s)", syncFieldDocsURL)
	}

	once.Do(warnMessage)
	dev.Syncs = append(
		dev.Syncs,
		Sync{
			LocalPath:  filepath.Join(".", dev.SubPath),
			RemotePath: dev.MountPath,
		},
	)
	return nil
}

func (dev *Dev) translateDeprecatedWorkdir(main *Dev) error {
	if dev.WorkDir == "" || len(dev.Syncs) > 0 {
		return nil
	}
	if main != nil && main.MountPath == "" {
		return fmt.Errorf("'workdir' is not supported to define your synchronized folders in 'services'. Use the field 'sync' instead (%s)", syncFieldDocsURL)
	}

	dev.MountPath = dev.WorkDir
	dev.Syncs = append(
		dev.Syncs,
		Sync{
			LocalPath:  filepath.Join(".", dev.SubPath),
			RemotePath: dev.WorkDir,
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
		dev.Syncs = append(dev.Syncs, Sync{LocalPath: v.LocalPath, RemotePath: v.RemotePath})
	}
	dev.Volumes = volumes
}

//IsSubPathFolder checks if a sync folder is a suboath of another sync folder
func (dev *Dev) IsSubPathFolder(path string) (bool, error) {
	found := false
	for _, sync := range dev.Syncs {
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

func getDataSubPath(path string) string {
	return filepath.ToSlash(filepath.Join(DataSubPath, path))
}

func getSourceSubPath(path string) string {
	path = path[len(filepath.VolumeName(path)):]
	return filepath.ToSlash(filepath.Join(SourceCodeSubPath, path))
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
	if len(dev.Volumes) > 0 {
		return fmt.Errorf("'persistentVolume.enabled' must be set to true to use volumes")
	}
	for _, sync := range dev.Syncs {
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
	for _, sync := range dev.Syncs {
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
	for _, sync := range dev.Syncs {
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
	for _, sync := range dev.Syncs {
		_, err := main.IsSubPathFolder(sync.LocalPath)
		if err != nil {
			if err == errors.ErrNotFound {
				return fmt.Errorf("LocalPath '%s' in 'services' not defined in the field 'sync' of the main development container", sync.LocalPath)
			}
			return err
		}
	}
	return nil
}

func (dev *Dev) validateVolumes(main *Dev) error {
	if len(dev.Syncs) == 0 {
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
