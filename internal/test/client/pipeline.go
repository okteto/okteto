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

package client

import (
	"context"
	"time"

	"github.com/okteto/okteto/pkg/types"
)

// FakePipelineClient mocks the namespace interface
type FakePipelineClient struct {
	responses *FakePipelineResponses
}

// FakePipelineResponses represents the responses of the API
type FakePipelineResponses struct {
	DeployErr   error
	ResourceErr error
	WaitErr     error
	DestroyErr  error

	DeployResponse  *types.GitDeployResponse
	DestroyResponse *types.GitDeployResponse
	ResourcesMap    map[string]string
	DeployOpts      types.PipelineDeployOptions
	CallCount       int
}

// NewFakePipelineClient creates a pipeline client to use in tests
func NewFakePipelineClient(responses *FakePipelineResponses) *FakePipelineClient {
	return &FakePipelineClient{
		responses: responses,
	}
}

// Deploy deploys a fake pipeline
func (fc *FakePipelineClient) Deploy(_ context.Context, opts types.PipelineDeployOptions) (*types.GitDeployResponse, error) {
	fc.responses.DeployOpts = opts
	fc.responses.CallCount++
	return fc.responses.DeployResponse, fc.responses.DeployErr
}

// WaitForActionToFinish waits for a pipeline to finish
func (fc *FakePipelineClient) WaitForActionToFinish(_ context.Context, _, _, _ string, _ time.Duration) error {
	return fc.responses.WaitErr
}

// Destroy destroys a pipeline
func (fc *FakePipelineClient) Destroy(_ context.Context, _, _ string, _ bool) (*types.GitDeployResponse, error) {
	return fc.responses.DestroyResponse, fc.responses.DestroyErr
}

// GetResourcesStatus gets the status of the resources from a pipeline name
func (fc *FakePipelineClient) GetResourcesStatus(_ context.Context, _, _ string) (map[string]string, error) {
	return fc.responses.ResourcesMap, fc.responses.ResourceErr
}

// GetByName returns the name of the pipeline
func (*FakePipelineClient) GetByName(_ context.Context, _, _ string) (*types.GitDeploy, error) {
	return nil, nil
}

// WaitForActionProgressing waits for a pipeline to start progressing
func (fc *FakePipelineClient) WaitForActionProgressing(_ context.Context, _, _, _ string, _ time.Duration) error {
	return fc.responses.WaitErr
}
