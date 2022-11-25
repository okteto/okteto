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

package types

import (
	"context"
	"time"
)

// OktetoInterface represents the client that connects to the backend to create API calls
type OktetoInterface interface {
	User() UserInterface
	Namespaces() NamespaceInterface
	Previews() PreviewInterface
	Pipeline() PipelineInterface
	Stream() StreamInterface
}

// UserInterface represents the client that connects to the user functions
type UserInterface interface {
	GetContext(ctx context.Context) (*UserContext, error)
}

// NamespaceInterface represents the client that connects to the namespace functions
type NamespaceInterface interface {
	Create(ctx context.Context, namespace string) (string, error)
	List(ctx context.Context) ([]Namespace, error)
	Delete(ctx context.Context, namespace string) error
	AddMembers(ctx context.Context, namespace string, members []string) error
	SleepNamespace(ctx context.Context, namespace string) error
	DestroyAll(ctx context.Context, namespace string) error
}

// PreviewInterface represents the client that connects to the preview functions
type PreviewInterface interface {
	List(ctx context.Context) ([]Preview, error)
	DeployPreview(ctx context.Context, name, scope, repository, branch, sourceUrl, filename string, variables []Variable) (*PreviewResponse, error)
	GetResourcesStatusFromPreview(ctx context.Context, previewName, devName string) (map[string]string, error)
}

// PipelineInterface represents the client that connects to the pipeline functions
type PipelineInterface interface {
	Deploy(ctx context.Context, opts PipelineDeployOptions) (*GitDeployResponse, error)
	WaitForActionToFinish(ctx context.Context, pipelineName, namespace, actionName string, timeout time.Duration) error
	Destroy(ctx context.Context, name, namespace string, destroyVolumes bool) (*GitDeployResponse, error)
	GetResourcesStatus(ctx context.Context, name, namespace string) (map[string]string, error)
	GetByName(ctx context.Context, name, namespace string) (*GitDeploy, error)
	WaitForActionProgressing(ctx context.Context, pipelineName, namespace, actionName string, timeout time.Duration) error
}

// OktetoClientProvider provides an okteto client ready to use or fail
type OktetoClientProvider interface {
	Provide() (OktetoInterface, error)
}

// StreamInterface represents the streaming client
type StreamInterface interface {
	PipelineLogs(ctx context.Context, name, namespace, actionName string) error
}
