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

	dockertypes "github.com/docker/cli/cli/config/types"
	"github.com/okteto/okteto/pkg/types"
)

// FakeUserClient is used to mock the userClient interface
type FakeUserClient struct {
	userCtx           *types.UserContext
	userSecrets       []types.Secret
	err               []error
	errGetUserSecrets error
}

func NewFakeUsersClient(user *types.User, err ...error) *FakeUserClient {
	return &FakeUserClient{userCtx: &types.UserContext{User: *user}, err: err}
}

func NewFakeUsersClientWithContext(userCtx *types.UserContext, err ...error) *FakeUserClient {
	return &FakeUserClient{userCtx: userCtx, err: err}
}

func (c *FakeUserClient) GetContext(_ context.Context, _ string) (*types.UserContext, error) {
	if c.err != nil && len(c.err) > 0 {
		err := c.err[0]
		c.err = c.err[1:]
		if err != nil {
			return nil, err
		}
	}

	return c.userCtx, nil
}

func (c *FakeUserClient) GetUserSecrets(_ context.Context) ([]types.Secret, error) {
	if c.errGetUserSecrets != nil {
		return nil, c.errGetUserSecrets
	}
	return c.userSecrets, nil
}

func (*FakeUserClient) GetClusterCertificate(_ context.Context, _, _ string) ([]byte, error) {
	return nil, nil
}

func (*FakeUserClient) GetClusterMetadata(_ context.Context, _ string) (types.ClusterMetadata, error) {
	return types.ClusterMetadata{}, nil
}

func (*FakeUserClient) GetRegistryCredentials(_ context.Context, _ string) (dockertypes.AuthConfig, error) {
	return dockertypes.AuthConfig{}, nil
}
