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

package environment

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/registry"
)

type imageReferenceGetter interface {
	GetImageReference(reference string) (registry.OktetoImageReference, error)
}

type ServiceEnvVarsSetter struct {
	buildEnvironments    map[string]string
	lock                 sync.RWMutex
	ioCtrl               *io.Controller
	imageReferenceGetter imageReferenceGetter
}

func NewServiceEnvVarsSetter(ioCtrl *io.Controller, imageReferenceGetter imageReferenceGetter) *ServiceEnvVarsSetter {
	return &ServiceEnvVarsSetter{
		lock:                 sync.RWMutex{},
		ioCtrl:               ioCtrl,
		imageReferenceGetter: imageReferenceGetter,
		buildEnvironments:    make(map[string]string),
	}
}

// SetEnvVar sets an environment variable
func (es *ServiceEnvVarsSetter) SetEnvVar(key, value string) {
	es.lock.Lock()
	es.buildEnvironments[key] = value
	os.Setenv(key, value)
	es.lock.Unlock()
}

// SetServiceEnvVars set okteto build env vars
func (es *ServiceEnvVarsSetter) SetServiceEnvVars(service, reference string) {
	ref, err := es.imageReferenceGetter.GetImageReference(reference)
	if err != nil {
		es.ioCtrl.Logger().Debugf("could not set service env vars: %s", err)
		return
	}

	es.ioCtrl.Logger().Debugf("envs registry=%s repository=%s image=%s tag=%s", ref.Registry, ref.Repo, ref.Image, ref.Tag)

	// Can't add env vars with -
	sanitizedSvc := strings.ToUpper(strings.ReplaceAll(service, "-", "_"))

	registryKey := fmt.Sprintf("OKTETO_BUILD_%s_REGISTRY", sanitizedSvc)
	es.lock.Lock()
	es.buildEnvironments[registryKey] = ref.Registry
	os.Setenv(registryKey, ref.Registry)
	es.lock.Unlock()

	repositoryKey := fmt.Sprintf("OKTETO_BUILD_%s_REPOSITORY", sanitizedSvc)
	es.lock.Lock()
	es.buildEnvironments[repositoryKey] = ref.Repo
	os.Setenv(repositoryKey, ref.Repo)
	es.lock.Unlock()

	imageKey := fmt.Sprintf("OKTETO_BUILD_%s_IMAGE", sanitizedSvc)
	es.lock.Lock()
	es.buildEnvironments[imageKey] = ref.Image
	os.Setenv(imageKey, ref.Image)
	es.lock.Unlock()

	tagKey := fmt.Sprintf("OKTETO_BUILD_%s_TAG", sanitizedSvc)
	es.lock.Lock()
	es.buildEnvironments[tagKey] = ref.Tag
	os.Setenv(tagKey, ref.Tag)
	es.lock.Unlock()

	sha := ref.Tag
	if strings.HasPrefix(sha, "sha256:") {
		sha = fmt.Sprintf("%s@%s", model.OktetoDefaultImageTag, sha)
	}
	shaKey := fmt.Sprintf("OKTETO_BUILD_%s_SHA", sanitizedSvc)
	es.lock.Lock()
	es.buildEnvironments[shaKey] = sha
	os.Setenv(shaKey, sha)
	es.lock.Unlock()

	es.ioCtrl.Logger().Debug("manifest env vars set")
}

// GetBuildEnvVars gets okteto build env vars
func (es *ServiceEnvVarsSetter) GetBuildEnvVars() map[string]string {
	return es.buildEnvironments
}
