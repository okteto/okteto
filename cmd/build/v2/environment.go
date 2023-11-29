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

package v2

import (
	"fmt"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/model"
)

// SetServiceEnvVars set okteto build env vars
func (bc *OktetoBuilder) SetServiceEnvVars(service, reference string) {
	ref, err := bc.Registry.GetImageReference(reference)
	if err != nil {
		bc.ioCtrl.Logger().Debugf("could not set service env vars: %s", err)
		return
	}

	bc.ioCtrl.Logger().Debugf("envs registry=%s repository=%s image=%s tag=%s", ref.Registry, ref.Repo, ref.Image, ref.Tag)

	// Can't add env vars with -
	sanitizedSvc := strings.ToUpper(strings.ReplaceAll(service, "-", "_"))

	registryKey := fmt.Sprintf("OKTETO_BUILD_%s_REGISTRY", sanitizedSvc)
	bc.lock.Lock()
	bc.buildEnvironments[registryKey] = ref.Registry
	os.Setenv(registryKey, ref.Registry)
	bc.lock.Unlock()

	repositoryKey := fmt.Sprintf("OKTETO_BUILD_%s_REPOSITORY", sanitizedSvc)
	bc.lock.Lock()
	bc.buildEnvironments[repositoryKey] = ref.Repo
	os.Setenv(repositoryKey, ref.Repo)
	bc.lock.Unlock()

	imageKey := fmt.Sprintf("OKTETO_BUILD_%s_IMAGE", sanitizedSvc)
	bc.lock.Lock()
	bc.buildEnvironments[imageKey] = ref.Image
	os.Setenv(imageKey, ref.Image)
	bc.lock.Unlock()

	tagKey := fmt.Sprintf("OKTETO_BUILD_%s_TAG", sanitizedSvc)
	bc.lock.Lock()
	bc.buildEnvironments[tagKey] = ref.Tag
	os.Setenv(tagKey, ref.Tag)
	bc.lock.Unlock()

	sha := ref.Tag
	if strings.HasPrefix(sha, "sha256:") {
		sha = fmt.Sprintf("%s@%s", model.OktetoDefaultImageTag, sha)
	}
	shaKey := fmt.Sprintf("OKTETO_BUILD_%s_SHA", sanitizedSvc)
	bc.lock.Lock()
	bc.buildEnvironments[shaKey] = sha
	os.Setenv(shaKey, sha)
	bc.lock.Unlock()

	bc.ioCtrl.Logger().Debug("manifest env vars set")
}

// GetBuildEnvVars gets okteto build env vars
func (bc *OktetoBuilder) GetBuildEnvVars() map[string]string {
	return bc.buildEnvironments
}
