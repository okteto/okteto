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
	"errors"
	"fmt"
	"os"
	"strings"

	oktetoError "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/constants"
	"github.com/okteto/okteto/pkg/model/files"
	"github.com/okteto/okteto/pkg/model/secrets"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	errBadName = fmt.Errorf("Invalid name: must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character")
)

//Validate checks if a dev is a valid dev
func (dev *Dev) Validate() error {
	if dev.Name == "" {
		return fmt.Errorf("Name cannot be empty")
	}

	if files.ValidKubeNameRegex.MatchString(dev.Name) {
		return errBadName
	}

	if strings.HasPrefix(dev.Name, "-") || strings.HasSuffix(dev.Name, "-") {
		return errBadName
	}

	if err := validatePullPolicy(dev.ImagePullPolicy); err != nil {
		return err
	}

	if err := validateSecrets(dev.Secrets); err != nil {
		return err
	}
	if err := dev.validateSecurityContext(); err != nil {
		return err
	}
	if err := dev.validatePersistentVolume(); err != nil {
		return err
	}

	if err := dev.validateVolumes(nil); err != nil {
		return err
	}

	if err := dev.validateExternalVolumes(); err != nil {
		return err
	}

	if err := dev.validateSync(); err != nil {
		return err
	}

	if _, err := resource.ParseQuantity(dev.PersistentVolumeSize()); err != nil {
		return fmt.Errorf("'persistentVolume.size' is not valid. A sample value would be '10Gi'")
	}

	if dev.SSHServerPort <= 0 {
		return fmt.Errorf("'sshServerPort' must be > 0")
	}

	for _, s := range dev.Services {
		if err := validatePullPolicy(s.ImagePullPolicy); err != nil {
			return err
		}
		if err := s.validateVolumes(dev); err != nil {
			return err
		}
	}

	if dev.Docker.Enabled && !dev.PersistentVolumeEnabled() {
		log.Information(constants.ManifestDockerDocsURL)
		return fmt.Errorf("Docker support requires persistent volume to be enabled")
	}

	return nil
}

func (dev *Dev) validateSync() error {
	for _, folder := range dev.Sync.Folders {
		validPath, err := os.Stat(folder.LocalPath)

		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return oktetoError.UserError{
					E:    fmt.Errorf("path '%s' does not exist", folder.LocalPath),
					Hint: "Update the `sync` field in your okteto manifest file to a valid directory path.",
				}
			}

			return oktetoError.UserError{
				E:    fmt.Errorf("File paths are not supported on sync fields"),
				Hint: "Update the `sync` field in your okteto manifest file to a valid directory path.",
			}
		}

		if !validPath.IsDir() {
			return oktetoError.UserError{
				E:    fmt.Errorf("File paths are not supported on sync fields"),
				Hint: "Update the `sync` field in your okteto manifest file to a valid directory path.",
			}
		}

	}
	return nil
}

func validatePullPolicy(pullPolicy apiv1.PullPolicy) error {
	switch pullPolicy {
	case apiv1.PullAlways:
	case apiv1.PullIfNotPresent:
	case apiv1.PullNever:
	default:
		return fmt.Errorf("supported values for 'imagePullPolicy' are: 'Always', 'IfNotPresent' or 'Never'")
	}
	return nil
}

func validateSecrets(secrets []secrets.Secret) error {
	seen := map[string]bool{}
	for _, s := range secrets {
		if _, ok := seen[s.GetFileName()]; ok {
			return fmt.Errorf("Secrets with the same basename '%s' are not supported", s.GetFileName())
		}
		seen[s.GetFileName()] = true
	}
	return nil
}

// validateSecurityContext checks to see if a root user is specified with runAsNonRoot enabled
func (dev *Dev) validateSecurityContext() error {
	if dev.isRootUser() && dev.RunAsNonRoot() {
		return fmt.Errorf("Running as the root user breaks runAsNonRoot constraint of the securityContext")
	}
	return nil
}

// isRootUser returns true if a root user is specified
func (dev *Dev) isRootUser() bool {
	if dev.SecurityContext == nil {
		return false
	}
	if dev.SecurityContext.RunAsUser == nil {
		return false
	}
	return *dev.SecurityContext.RunAsUser == 0
}

// RunAsNonRoot returns true if the development container must run as a non-root user
func (dev *Dev) RunAsNonRoot() bool {
	if dev.SecurityContext == nil {
		return false
	}
	if dev.SecurityContext.RunAsNonRoot == nil {
		return false
	}
	return *dev.SecurityContext.RunAsNonRoot
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
			if err == oktetoError.ErrNotFound {
				return fmt.Errorf("LocalPath '%s' in 'services' not defined in the field 'sync' of the main development container", sync.LocalPath)
			}
			return err
		}
	}
	return nil
}

func (dev *Dev) validateVolumes(main *Dev) error {
	if len(dev.Sync.Folders) == 0 {
		return fmt.Errorf("the 'sync' field is mandatory. More info at %s", constants.SyncFieldDocsURL)
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

//ValidateForExtraFields validates that the dev.services does not have a extra field

func (service *Dev) ValidateForExtraFields() error {
	docURL := fmt.Sprintf("Please visit %s for documentation", constants.ManifestSvcsDocsURL)
	errorMessage := "%q is not supported in Services." + docURL
	if service.Username != "" {
		return fmt.Errorf(errorMessage, "username")
	}
	if service.RegistryURL != "" {
		return fmt.Errorf(errorMessage, "registryURL")
	}
	if service.Autocreate {
		return fmt.Errorf(errorMessage, "autocreate")
	}
	if service.Context != "" {
		return fmt.Errorf(errorMessage, "context")
	}
	if service.Push != nil {
		return fmt.Errorf(errorMessage, "push")
	}
	if service.Secrets != nil {
		return fmt.Errorf(errorMessage, "secrets")
	}
	if service.Healthchecks {
		return fmt.Errorf(errorMessage, "healthchecks")
	}
	if service.Probes != nil {
		return fmt.Errorf(errorMessage, "probes")
	}
	if service.Lifecycle != nil {
		return fmt.Errorf(errorMessage, "lifecycle")
	}
	if service.SecurityContext != nil {
		return fmt.Errorf(errorMessage, "securityContext")
	}
	if service.ServiceAccount != "" {
		return fmt.Errorf(errorMessage, "serviceAccount")
	}
	if service.RemotePort != 0 {
		return fmt.Errorf(errorMessage, "remote")
	}
	if service.SSHServerPort != 0 {
		return fmt.Errorf(errorMessage, "sshServerPort")
	}
	if service.ExternalVolumes != nil {
		return fmt.Errorf(errorMessage, "externalVolumes")
	}
	if service.parentSyncFolder != "" {
		return fmt.Errorf(errorMessage, "parentSyncFolder")
	}
	if service.Forward != nil {
		return fmt.Errorf(errorMessage, "forward")
	}
	if service.Reverse != nil {
		return fmt.Errorf(errorMessage, "reverse")
	}
	if service.Interface != "" {
		return fmt.Errorf(errorMessage, "interface")
	}
	if service.Services != nil {
		return fmt.Errorf(errorMessage, "services")
	}
	if service.PersistentVolumeInfo != nil {
		return fmt.Errorf(errorMessage, "persistentVolume")
	}
	if service.InitContainer.Image != "" {
		return fmt.Errorf(errorMessage, "initContainer")
	}
	if service.InitFromImage {
		return fmt.Errorf(errorMessage, "initFromImage")
	}
	if service.Timeout != (Timeout{}) {
		return fmt.Errorf(errorMessage, "timeout")
	}
	if service.Docker.Enabled && service.Docker.Image != "" {
		return fmt.Errorf(errorMessage, "docker")
	}
	if service.Divert != nil {
		return fmt.Errorf(errorMessage, "divert")
	}

	return nil
}
