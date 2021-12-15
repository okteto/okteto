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

type fakeUserClient struct {
	userCtx *types.UserContext
	err     error
}

func NewFakeUsersClient(user *types.User, err error) *fakeUserClient {
	return &fakeUserClient{userCtx: &types.UserContext{User: *user}, err: err}
}

func (c *fakeUserClient) GetContext(ctx context.Context) (*types.UserContext, error) {
	return c.userCtx, c.err
}
