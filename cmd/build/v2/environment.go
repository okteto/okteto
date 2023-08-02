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

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

// SetServiceEnvVars set okteto build env vars
func (bc *OktetoBuilder) SetServiceEnvVars(service, reference string) {
	ref, err := bc.Registry.GetImageReference(reference)
	if err != nil {
		oktetoLog.Debugf("could not set service env vars: %w", err)
		return
	}

	oktetoLog.Debugf("envs registry=%s repository=%s image=%s tag=%s", ref.Registry, ref.Repo, ref.Image, ref.Tag)

	// Can't add env vars with -
	sanitizedSvc := strings.ToUpper(strings.ReplaceAll(service, "-", "_"))

	registryKey := fmt.Sprintf("OKTETO_BUILD_%s_REGISTRY", sanitizedSvc)
	bc.lock.Lock()
	bc.buildEnvironments[registryKey] = ref.Registry
	err = os.Setenv(registryKey, ref.Registry)
	if err != nil {
		oktetoLog.Debugf("error to set registry env for service '%s'", service)
	}
	bc.lock.Unlock()

	repositoryKey := fmt.Sprintf("OKTETO_BUILD_%s_REPOSITORY", sanitizedSvc)
	bc.lock.Lock()
	bc.buildEnvironments[repositoryKey] = ref.Repo
	err = os.Setenv(repositoryKey, ref.Repo)
	if err != nil {
		oktetoLog.Debugf("error to set repository env for service '%s'", service)
	}
	bc.lock.Unlock()

	imageKey := fmt.Sprintf("OKTETO_BUILD_%s_IMAGE", sanitizedSvc)
	bc.lock.Lock()
	bc.buildEnvironments[imageKey] = ref.Image
	err = os.Setenv(imageKey, ref.Image)
	if err != nil {
		oktetoLog.Debugf("error to set image env for service '%s'", service)
	}
	bc.lock.Unlock()

	tagKey := fmt.Sprintf("OKTETO_BUILD_%s_TAG", sanitizedSvc)
	bc.lock.Lock()
	bc.buildEnvironments[tagKey] = ref.Tag
	err = os.Setenv(tagKey, ref.Tag)
	if err != nil {
		oktetoLog.Debugf("error to set tag env for service '%s'", service)
	}
	bc.lock.Unlock()

	sha := ref.Tag
	if strings.HasPrefix(sha, "sha256:") {
		sha = fmt.Sprintf("%s@%s", model.OktetoDefaultImageTag, sha)
	}
	shaKey := fmt.Sprintf("OKTETO_BUILD_%s_SHA", sanitizedSvc)
	bc.lock.Lock()
	bc.buildEnvironments[shaKey] = sha
	err = os.Setenv(shaKey, sha)
	if err != nil {
		oktetoLog.Debugf("error to set sha env for service '%s'", service)
	}
	bc.lock.Unlock()

	oktetoLog.Debug("manifest env vars set")
}

// GetBuildEnvVars gets okteto build env vars
func (bc *OktetoBuilder) GetBuildEnvVars() map[string]string {
	return bc.buildEnvironments
}
