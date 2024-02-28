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

package apps

import (
	"fmt"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
)

func ValidateMountPaths(spec *apiv1.PodSpec, dev *model.Dev) error {
	if dev.PersistentVolumeInfo == nil || !dev.PersistentVolumeInfo.Enabled {
		return nil
	}
	devContainer := GetDevContainer(spec, dev.Container)
	for _, vm := range devContainer.VolumeMounts {
		if dev.GetVolumeName() == vm.Name {
			continue
		}
		for _, syncVolume := range dev.Sync.Folders {
			if vm.MountPath == syncVolume.RemotePath {
				return oktetoErrors.UserError{
					E:    fmt.Errorf("'%s' is already defined as volume in %s", vm.MountPath, dev.Name),
					Hint: `Disable the okteto persistent volume (https://okteto.com/docs/reference/okteto-manifest/#persistentvolume-object-optional) and try again`}
			}
		}
	}
	return nil
}
