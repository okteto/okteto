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
	"fmt"

	"github.com/okteto/okteto/pkg/types"
)

type fakeNamespaceClient struct {
	namespaces []types.Namespace
	err        error
}

func NewFakeNamespaceClient(ns []types.Namespace, err error) *fakeNamespaceClient {
	return &fakeNamespaceClient{namespaces: ns, err: err}
}

// CreateNamespace creates a namespace
func (c *fakeNamespaceClient) Create(ctx context.Context, namespace string) (string, error) {
	c.namespaces = append(c.namespaces, types.Namespace{ID: namespace})
	return namespace, c.err
}

// List list namespaces
func (c *fakeNamespaceClient) List(ctx context.Context) ([]types.Namespace, error) {
	return c.namespaces, c.err
}

// AddNamespaceMembers adds members to a namespace
func (c *fakeNamespaceClient) AddMembers(ctx context.Context, namespace string, members []string) error {
	return c.err
}

// DeleteNamespace deletes a namespace
func (c *fakeNamespaceClient) Delete(ctx context.Context, namespace string) error {
	toRemove := -1
	for idx, ns := range c.namespaces {
		if ns.ID == namespace {
			toRemove = idx
			break
		}
	}
	if toRemove != -1 {
		c.namespaces[toRemove] = c.namespaces[len(c.namespaces)-1]
		c.namespaces = c.namespaces[:len(c.namespaces)-1]
		return nil
	}
	return fmt.Errorf("not found")
}
