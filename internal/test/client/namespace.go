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

//FakeNamespaceClient mocks the namespace interface
type FakeNamespaceClient struct {
	namespaces []types.Namespace
	err        error
}

func NewFakeNamespaceClient(ns []types.Namespace, err error) *FakeNamespaceClient {
	return &FakeNamespaceClient{namespaces: ns, err: err}
}

// CreateNamespace creates a namespace
func (c *FakeNamespaceClient) Create(_ context.Context, namespace string) (string, error) {
	c.namespaces = append(c.namespaces, types.Namespace{ID: namespace})
	return namespace, c.err
}

// List list namespaces
func (c *FakeNamespaceClient) List(_ context.Context) ([]types.Namespace, error) {
	return c.namespaces, c.err
}

// AddNamespaceMembers adds members to a namespace
func (c *FakeNamespaceClient) AddMembers(_ context.Context, _ string, _ []string) error {
	return c.err
}

// DeleteNamespace deletes a namespace
func (c *FakeNamespaceClient) Delete(_ context.Context, namespace string) error {
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

// SleepNamespace deletes a namespace
func (c *FakeNamespaceClient) SleepNamespace(_ context.Context, namespace string) error {
	return nil
}
