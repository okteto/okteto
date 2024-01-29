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

package namespace

import (
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type fakeK8sProvider struct {
	k8sClient kubernetes.Interface
}

func (p *fakeK8sProvider) Provide(_ *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error) {
	return p.k8sClient, nil, nil
}

func (f *fakeK8sProvider) ProvideWithLogger(c *clientcmdapi.Config, _ *io.K8sLogger) (kubernetes.Interface, *rest.Config, error) {
	return f.Provide(c)
}

func (*fakeK8sProvider) GetIngressClient() (*ingresses.Client, error) {
	return nil, nil
}

func newFakeContextCommand(c *client.FakeOktetoClient, user *types.User) *contextCMD.Command {
	cmd := contextCMD.NewContextCommand()
	cmd.OktetoClientProvider = client.NewFakeOktetoClientProvider(c)
	cmd.K8sClientProvider = test.NewFakeK8sProvider(nil)
	cmd.LoginController = test.NewFakeLoginController(user, nil)
	cmd.OktetoContextWriter = test.NewFakeOktetoContextWriter()
	return cmd
}

func NewFakeNamespaceCommand(okClient *client.FakeOktetoClient, k8sClient kubernetes.Interface, user *types.User) *Command {
	return &Command{
		okClient: okClient,
		ctxCmd:   newFakeContextCommand(okClient, user),
		k8sClientProvider: &fakeK8sProvider{
			k8sClient: k8sClient,
		},
	}
}
