// Copyright 2020 The Okteto Authors
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

package labels

const (
	//Version represents the current dev data version
	Version = "1.0"

	// DevLabel indicates the dev pod
	DevLabel = "dev.okteto.com"

	// InteractiveDevLabel indicates the interactive dev pod
	InteractiveDevLabel = "interactive.dev.okteto.com"

	// DetachedDevLabel indicates the detached dev pods
	DetachedDevLabel = "detached.dev.okteto.com"

	// RevisionAnnotation indicates the revision when the development environment was activated
	RevisionAnnotation = "dev.okteto.com/revision"

	// DeploymentAnnotation indicates the original deployment manifest  when the development environment was activated
	DeploymentAnnotation = "dev.okteto.com/deployment"

	// TranslationAnnotation sets the translation rules
	TranslationAnnotation = "dev.okteto.com/translation"

	// SyncLabel indicates a synthing pod
	SyncLabel = "syncthing.okteto.com"
)
