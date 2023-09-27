package namespace

import (
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
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

func (*fakeK8sProvider) GetIngressClient() (*ingresses.Client, error) {
	return nil, nil
}

func newFakeContextCommand(c *client.FakeOktetoClient, user *types.User) *contextCMD.ContextCommand {
	cmd := contextCMD.NewContextCommand()
	cmd.OktetoClientProvider = client.NewFakeOktetoClientProvider(c)
	cmd.K8sClientProvider = test.NewFakeK8sProvider(nil)
	cmd.LoginController = test.NewFakeLoginController(user, nil)
	cmd.OktetoContextWriter = test.NewFakeOktetoContextWriter()
	return cmd
}

func NewFakeNamespaceCommand(okClient *client.FakeOktetoClient, k8sClient kubernetes.Interface, user *types.User) *NamespaceCommand {
	return &NamespaceCommand{
		okClient: okClient,
		ctxCmd:   newFakeContextCommand(okClient, user),
		k8sClientProvider: &fakeK8sProvider{
			k8sClient: k8sClient,
		},
	}
}
