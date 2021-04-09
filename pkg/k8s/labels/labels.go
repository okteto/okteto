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

import (
	"fmt"
	"strings"
)

const (
	//Version represents the current dev data version
	Version = "1.0"

	// TimeFormat is the format to use when storing timestamps as a string
	TimeFormat = "2006-01-02T15:04:05"

	// DevLabel indicates the dev pod
	DevLabel = "dev.okteto.com"

	// InteractiveDevLabel indicates the interactive dev pod
	InteractiveDevLabel = "interactive.dev.okteto.com"

	// DetachedDevLabel indicates the detached dev pods
	DetachedDevLabel = "detached.dev.okteto.com"

	// RevisionAnnotation indicates the revision when the development container was activated
	RevisionAnnotation = "dev.okteto.com/revision"

	// DeploymentAnnotation indicates the original deployment manifest  when the development container was activated
	DeploymentAnnotation = "dev.okteto.com/deployment"

	// LastBuiltAnnotation indicates the timestamp of an operation
	LastBuiltAnnotation = "dev.okteto.com/last-built"

	// TranslationAnnotation sets the translation rules
	TranslationAnnotation = "dev.okteto.com/translation"

	// SyncLabel indicates a syncthing pod
	SyncLabel = "syncthing.okteto.com"

	//OktetoRepositoryAnnotation indicates the git repo url with the source code of this component
	OktetoRepositoryAnnotation = "dev.okteto.com/repository"

	//OktetoPathAnnotation indicates the okteto manifest path of this component
	OktetoPathAnnotation = "dev.okteto.com/path"

	//FluxAnnotation indicates if the deployment ha been deployed by Flux
	FluxAnnotation = "helm.fluxcd.io/antecedent"

	//DefaultStorageClassAnnotation indicates the defaault storage class
	DefaultStorageClassAnnotation = "storageclass.kubernetes.io/is-default-class"

	//StateBeforeSleepingAnnontation indicates the state of the resource prior to scale it to zero
	StateBeforeSleepingAnnontation = "dev.okteto.com/state-before-sleeping"

	// DeployedByLabel indicates the service account that deployed an object
	DeployedByLabel = "dev.okteto.com/deployed-by"

	// StackLabel indicates the object is a stack
	StackLabel = "stack.okteto.com"

	// StackNameLabel indicates the name of the stack an object belongs to
	StackNameLabel = "stack.okteto.com/name"

	// StackServiceNameLabel indicates the name of the stack service an object belongs to
	StackServiceNameLabel = "stack.okteto.com/service"

	// OktetoAutoIngressAnnotation indicates an ingress must be created for a service
	OktetoAutoIngressAnnotation = "dev.okteto.com/auto-ingress"

	// OktetoInstallerRunningLabel indicates the okteto installer is running on this resource
	OktetoInstallerRunningLabel = "dev.okteto.com/installer-running"
)

//TransformLabelsToSelector transforms a map of labels into a string k8s selector
func TransformLabelsToSelector(labels map[string]string) string {
	labelList := make([]string, 0)
	for key, value := range labels {
		labelList = append(labelList, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(labelList, ",")
}
