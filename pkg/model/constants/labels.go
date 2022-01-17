// Copyright 2021 The Okteto Authors
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
	// DevLabel indicates the deployment is in dev mode
	DevLabel = "dev.okteto.com"

	// DevCloneLabel indicates it is a dev pod clone
	DevCloneLabel = "dev.okteto.com/clone"

	// InteractiveDevLabel indicates the interactive dev pod
	InteractiveDevLabel = "interactive.dev.okteto.com"

	// DetachedDevLabel indicates the detached dev pods
	DetachedDevLabel = "detached.dev.okteto.com"

	// SyncLabel indicates a syncthing pod
	SyncLabel = "syncthing.okteto.com"

	// DeployedByLabel indicates the service account that deployed an object
	DeployedByLabel = "dev.okteto.com/deployed-by"

	// StackLabel indicates the object is a stack
	StackLabel = "stack.okteto.com"

	// StackNameLabel indicates the name of the stack an object belongs to
	StackNameLabel = "stack.okteto.com/name"

	// StackServiceNameLabel indicates the name of the stack service an object belongs to
	StackServiceNameLabel = "stack.okteto.com/service"

	// StackEndpointNameLabel indicates the name of the endpoint an object belongs to
	StackEndpointNameLabel = "stack.okteto.com/endpoint"

	// OktetoInstallerRunningLabel indicates the okteto installer is running on this resource
	OktetoInstallerRunningLabel = "dev.okteto.com/installer-running"

	// StackVolumeNameLabel indicates the name of the stack volume an object belongs to
	StackVolumeNameLabel = "stack.okteto.com/volume"

	//OktetoDivertLabel indicates the object is a diverted version
	OktetoDivertLabel = "dev.okteto.com/divert"
)
