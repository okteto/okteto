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

package types

// ClusterMetadata represents the okteto cluster metadata
type ClusterMetadata struct {
	ServerName          string
	PipelineRunnerImage string
	BuildKitInternalIP  string
	PublicDomain        string
	CompanyName         string
	CliImage            string
	SSHAgentInternalIP  string
	SSHAgentHostname    string
	SSHAgentPort        string
	CliMinVersion       string
	CliClusterVersion   string
	Certificate         []byte
	IsTrialLicense      bool
	DivertCRDSEnabled   bool
	GatewayName         string
	GatewayNamespace    string
}
