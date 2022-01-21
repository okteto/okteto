package okteto

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/k8s/ingresses"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var timeout time.Duration
var tOnce sync.Once

type K8sClientProvider interface {
	Provide(clientApiConfig *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error)
	GetIngressClient(ctx context.Context) (*ingresses.Client, error)
}

type K8sClient struct{}

func NewK8sClientProvider() *K8sClient {
	return &K8sClient{}
}

func (*K8sClient) Provide(clientApiConfig *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error) {
	return getK8sClientWithApiConfig(clientApiConfig)
}

func (*K8sClient) GetIngressClient(ctx context.Context) (*ingresses.Client, error) {
	c, _, err := GetK8sClient()
	if err != nil {
		return nil, err
	}
	iClient, err := ingresses.GetClient(ctx, c)
	if err != nil {
		return nil, err
	}
	return iClient, nil
}

func getKubernetesTimeout() time.Duration {
	tOnce.Do(func() {
		timeout = 0 * time.Second
		t, ok := os.LookupEnv(model.OktetoKubernetesTimeoutEnvVar)
		if !ok {
			return
		}

		parsed, err := time.ParseDuration(t)
		if err != nil {
			oktetoLog.Infof("'%s' is not a valid duration, ignoring", t)
			return
		}

		oktetoLog.Infof("OKTETO_KUBERNETES_TIMEOUT applied: '%s'", parsed.String())
		timeout = parsed
	})

	return timeout
}

func getK8sClientWithApiConfig(clientApiConfig *clientcmdapi.Config) (*kubernetes.Clientset, *rest.Config, error) {
	clientConfig := clientcmd.NewDefaultClientConfig(*clientApiConfig, nil)

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	config.Timeout = getKubernetesTimeout()

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return client, config, nil
}

func getDynamicClient(clientAPIConfig *clientcmdapi.Config) (dynamic.Interface, *rest.Config, error) {
	clientConfig := clientcmd.NewDefaultClientConfig(*clientAPIConfig, nil)

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	config.Timeout = getKubernetesTimeout()

	dc, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return dc, config, err
}

func getDiscoveryClient(clientAPIConfig *clientcmdapi.Config) (discovery.DiscoveryInterface, *rest.Config, error) {
	clientConfig := clientcmd.NewDefaultClientConfig(*clientAPIConfig, nil)

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	config.Timeout = getKubernetesTimeout()

	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return dc, config, err
}
