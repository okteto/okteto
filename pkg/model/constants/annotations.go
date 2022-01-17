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
	// AppReplicasAnnotation indicates the number of replicas before dev mode was activated
	AppReplicasAnnotation = "dev.okteto.com/replicas"

	// OktetoSampleAnnotation indicates that the repo is a okteto sample
	OktetoSampleAnnotation = "dev.okteto.com/sample"

	// DeploymentRevisionAnnotation indicates the revision when the development container was activated
	DeploymentRevisionAnnotation = "deployment.kubernetes.io/revision"

	// OktetoRevisionAnnotation indicates the revision when the development container was activated
	OktetoRevisionAnnotation = "dev.okteto.com/revision"

	// DeploymentAnnotation indicates the original deployment manifest  when the development container was activated
	DeploymentAnnotation = "dev.okteto.com/deployment"

	// StatefulsetAnnotation indicates the original statefulset manifest  when the development container was activated
	StatefulsetAnnotation = "dev.okteto.com/statefulset"

	// LastBuiltAnnotation indicates the timestamp of an operation
	LastBuiltAnnotation = "dev.okteto.com/last-built"

	// TranslationAnnotation sets the translation rules
	TranslationAnnotation = "dev.okteto.com/translation"

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

	// OktetoIngressAutoGenerateHost generates a ingress host for
	OktetoIngressAutoGenerateHost = "dev.okteto.com/generate-host"

	// OktetoAutoIngressAnnotation indicates an ingress must be created for a service
	OktetoAutoIngressAnnotation = "dev.okteto.com/auto-ingress"

	// OktetoPrivateSvcAnnotation indicates an ingress must be created private
	OktetoPrivateSvcAnnotation = "dev.okteto.com/private"

	//OktetoURLAnnotation indicates the okteto cluster public url
	OktetoURLAnnotation = "dev.okteto.com/url"

	//OktetoAutoCreateAnnotation indicates if the deployment was auto generatted by okteto up
	OktetoAutoCreateAnnotation = "dev.okteto.com/auto-create"

	//OktetoRestartAnnotation indicates the dev pod must be recreated to pull the latest version of its image
	OktetoRestartAnnotation = "dev.okteto.com/restart"

	//OktetoSyncAnnotation indicates the hash of the sync folders to force redeployment
	OktetoSyncAnnotation = "dev.okteto.com/sync"

	//OktetoStignoreAnnotation indicates the hash of the stignore files to force redeployment
	OktetoStignoreAnnotation = "dev.okteto.com/stignore"

	//OktetoDivertServiceModificationAnnotation indicates the service modification done by diverting a service
	OktetoDivertServiceModificationAnnotation = "divert.okteto.com/modification"

	//OktetoInjectTokenAnnotation annotation to inject the okteto token
	OktetoInjectTokenAnnotation = "dev.okteto.com/inject-token"
)
