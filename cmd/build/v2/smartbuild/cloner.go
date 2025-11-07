// Copyright 2025 The Okteto Authors
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

package smartbuild

import (
	"fmt"

	"github.com/okteto/okteto/pkg/log/io"
)

type cloner struct {
	registryController registryController
	ioCtrl             *io.Controller
}

func NewCloner(registryController registryController, ioCtrl *io.Controller) *cloner {
	return &cloner{
		registryController: registryController,
		ioCtrl:             ioCtrl,
	}
}

// CloneGlobalImageToDev clones the image from the global registry to the dev registry if needed
// if the built image belongs to global registry we clone it to the dev registry
// so that in can be used in dev containers (i.e. okteto up)
func (c *cloner) CloneGlobalImageToDev(globalImage, svcImage string) (string, error) {
	if c.registryController.IsGlobalRegistry(globalImage) {
		c.ioCtrl.Logger().Debugf("Copying image '%s' from global to personal registry", globalImage)
		if svcImage == "" {
			svcImage = c.registryController.GetDevImageFromGlobal(globalImage)
		}
		devImage, err := c.registryController.Clone(globalImage, svcImage)
		if err != nil {
			return "", fmt.Errorf("error cloning image '%s' to '%s': %w", globalImage, svcImage, err)
		}
		return devImage, nil
	}
	c.ioCtrl.Logger().Debugf("Image '%s' is not in the global registry", globalImage)
	return globalImage, nil
}
