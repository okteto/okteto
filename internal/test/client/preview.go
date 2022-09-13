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

	"github.com/okteto/okteto/pkg/types"
)

// FakePreviewsClient mocks the previews interface
type FakePreviewsClient struct {
	preview        *types.PreviewResponse
	previewList    []types.Preview
	resourceStatus map[string]string

	err error
}

// NewFakePreviewClient returns a new fake preview client
func NewFakePreviewClient(previewList []types.Preview, preview *types.PreviewResponse, err error) *FakePreviewsClient {
	return &FakePreviewsClient{
		previewList: previewList,
		preview:     preview,
		err:         err,
	}
}

// List list namespaces
func (c *FakePreviewsClient) List(_ context.Context) ([]types.Preview, error) {
	return c.previewList, c.err
}

// DeployPreview deploys a preview
func (c *FakePreviewsClient) DeployPreview(_ context.Context, _, _, _, _, _, _ string, _ []types.Variable) (*types.PreviewResponse, error) {
	return c.preview, c.err
}

// GetResourcesStatusFromPreview gets resources from a fake preview
func (c *FakePreviewsClient) GetResourcesStatusFromPreview(_ context.Context, _, _ string) (map[string]string, error) {
	return c.resourceStatus, c.err
}
