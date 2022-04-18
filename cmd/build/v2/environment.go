// Copyright 2022 The Okteto Authors
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
	"github.com/okteto/okteto/pkg/registry"
)

// SetServiceEnvVars set okteto build env vars
func (*OktetoBuilder) SetServiceEnvVars(service, reference string) {
	reg, repo, tag, image := registry.GetReferecenceEnvs(reference)

	oktetoLog.Debugf("envs registry=%s repository=%s image=%s tag=%s", reg, repo, image, tag)

	// Can't add env vars with -
	sanitizedSvc := strings.ToUpper(strings.ReplaceAll(service, "-", "_"))
	os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_REGISTRY", sanitizedSvc), reg)
	os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_REPOSITORY", sanitizedSvc), repo)
	os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_IMAGE", sanitizedSvc), reference)
	os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_TAG", sanitizedSvc), tag)

	oktetoLog.Debug("manifest env vars set")
	return
}
