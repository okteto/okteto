// Copyright 2026 The Okteto Authors
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

package deployable

import (
	"fmt"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

// GetGatewayEnvironment returns the gateway metadata exposed as env vars for custom commands.
func GetGatewayEnvironment() map[string]string {
	result := make(map[string]string)

	gateway := okteto.GetContext().Gateway
	if gateway == nil {
		return result
	}

	if gateway.Name != "" {
		result[model.OktetoDevGatewayNameEnvVar] = gateway.Name
	}
	if gateway.Namespace != "" {
		result[model.OktetoDevGatewayNamespaceEnvVar] = gateway.Namespace
	}

	return result
}

func appendGatewayEnvVars(envVars []string) []string {
	gatewayEnv := GetGatewayEnvironment()
	if value, ok := gatewayEnv[model.OktetoDevGatewayNameEnvVar]; ok {
		envVars = append(envVars, fmt.Sprintf("%s=%s", model.OktetoDevGatewayNameEnvVar, value))
	}
	if value, ok := gatewayEnv[model.OktetoDevGatewayNamespaceEnvVar]; ok {
		envVars = append(envVars, fmt.Sprintf("%s=%s", model.OktetoDevGatewayNamespaceEnvVar, value))
	}

	return envVars
}
