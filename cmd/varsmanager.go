// Copyright 2024 The Okteto Authors
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

package cmd

import (
	"github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/env"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/vars"
	"strings"
)

type VarsManager struct{}

func (*VarsManager) MaskVar(value string) {
	oktetoLog.AddMaskedWord(value)
}

func (*VarsManager) IsLocalVarSupportEnabled() bool {
	return env.LoadBooleanOrDefault(vars.OktetoSupportLocalVariablesEnabled, false)
}

func (*VarsManager) IsLocalVarException(v string) bool {
	if strings.HasPrefix(v, deploy.DependencyEnvVarPrefix) {
		return true
	}

	exceptions := map[string]bool{
		model.OktetoRegistryURLEnvVar:           true,
		model.OktetoBuildkitHostURLEnvVar:       true,
		model.OktetoBinEnvVar:                   true,
		model.OktetoSkipCleanupEnvVar:           true,
		model.OktetoUserEnvVar:                  true,
		model.OktetoUserNameEnvVar:              true,
		model.OktetoTokenEnvVar:                 true,
		model.OktetoURLEnvVar:                   true,
		model.OktetoContextEnvVar:               true,
		model.OktetoNamespaceEnvVar:             true,
		model.OktetoDomainEnvVar:                true,
		model.SyncthingVersionEnvVar:            true,
		model.OktetoSkipContextTestEnvVar:       true,
		model.OktetoAutoDeployEnvVar:            true,
		model.OktetoAppsSubdomainEnvVar:         true,
		model.OktetoPathEnvVar:                  true,
		model.OktetoExecuteSSHEnvVar:            true,
		model.OktetoSSHTimeoutEnvVar:            true,
		model.OktetoRescanIntervalEnvVar:        true,
		model.OktetoTimeoutEnvVar:               true,
		model.OktetoActionNameEnvVar:            true,
		model.OktetoComposeUpdateStrategyEnvVar: true,
		model.OktetoAutogenerateStignoreEnvVar:  true,

		model.DeprecatedOktetoCurrentDeployBelongsToPreviewEnvVar: true,

		constants.OktetoNameEnvVar:                       true,
		constants.OktetoSkipConfigCredentialsUpdate:      true,
		constants.OktetoHomeEnvVar:                       true,
		constants.KubeConfigEnvVar:                       true,
		constants.OktetoWithinDeployCommandContextEnvVar: true,
		constants.OktetoFolderEnvVar:                     true,
		constants.OktetoDeployRemote:                     true,
		constants.OktetoForceRemote:                      true,
		constants.OktetoTlsCertBase64EnvVar:              true,
		constants.OktetoInternalServerNameEnvVar:         true,
		constants.OktetoInvalidateCacheEnvVar:            true,
		constants.OktetoDeployRemoteImage:                true,
		constants.OktetoEnvFile:                          true,
		constants.OktetoGitBranchEnvVar:                  true,
		constants.OktetoGitCommitEnvVar:                  true,
		constants.OktetoDeployableEnvVar:                 true,
		constants.OktetoIsPreviewEnvVar:                  true,
	}

	if _, ok := exceptions[v]; ok {
		return true
	}

	return false
}
