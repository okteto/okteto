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
	DeployResponse *types.GitDeployResponse
	DeployErr      error

	WaitErr error

	DestroyResponse *types.GitDeployResponse
	DestroyErr      error

	ResourcesMap map[string]string
	ResourceErr  error
}

// NewFakePipelineClient creates a pipeline client to use in tests
func NewFakePipelineClient(responses *FakePipelineResponses) *FakePipelineClient {
	return &FakePipelineClient{
		responses: responses,
	}
}

// DeployPipeline deploys a fake pipeline
func (fc *FakePipelineClient) DeployPipeline(ctx context.Context, name, repository, branch, filename string, variables []types.Variable) (*types.GitDeployResponse, error) {
	return fc.responses.DeployResponse, fc.responses.DeployErr
}

// WaitForActionToFinish waits for a pipeline to finish
func (fc *FakePipelineClient) WaitForActionToFinish(ctx context.Context, pipelineName, actionName string, timeout time.Duration) error {
	return fc.responses.WaitErr
}

// DestroyPipeline destroys a pipeline
func (fc *FakePipelineClient) DestroyPipeline(ctx context.Context, name string, destroyVolumes bool) (*types.GitDeployResponse, error) {
	return fc.responses.DestroyResponse, fc.responses.DestroyErr
}

// GetResourcesStatusFromPipeline gets the status of the resources from a pipeline name
func (fc *FakePipelineClient) GetResourcesStatusFromPipeline(ctx context.Context, name string) (map[string]string, error) {
	return fc.responses.ResourcesMap, fc.responses.ResourceErr
}

// GetPipelineByName returns the name of the pipeline
func (fc *FakePipelineClient) GetPipelineByName(ctx context.Context, name string) (*types.GitDeploy, error) {
	return nil, nil
}
