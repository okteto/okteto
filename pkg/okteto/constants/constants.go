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
package constants

const (
	// OktetoNameEnvVar defines if the command is running inside okteot
	OktetoNameEnvVar = "OKTETO_NAME"

	// DevLabel indicates the deployment is in dev mode
	DevLabel = "dev.okteto.com"

	// OktetoURLAnnotation indicates the okteto cluster public url
	OktetoURLAnnotation = "dev.okteto.com/url"

	// OktetoExtension identifies the okteto extension in kubeconfig files
	OktetoExtension = "okteto"

	// OktetoSkipConfigCredentialsUpdate prevents the kubernetes config from being updated
	// with the okteto credentials
	OktetoSkipConfigCredentialsUpdate = "OKTETO_SKIP_CONFIG_CREDENTIALS_UPDATE"

	// OktetoHomeEnvVar defines the path of okteto folder
	OktetoHomeEnvVar = "OKTETO_HOME"
)
