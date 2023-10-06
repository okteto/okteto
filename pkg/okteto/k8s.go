package okteto

import (
	"errors"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/k8s/ingresses"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	timeout time.Duration
	tOnce   sync.Once

	// ErrK8sUnauthorised is returned when the kubernetes API call returns a 401 error
	ErrK8sUnauthorised = errors.New("k8s unauthorized error")
)

const (
	// oktetoKubernetesTimeoutEnvVar defines the timeout for kubernetes operations
	oktetoKubernetesTimeoutEnvVar = "OKTETO_KUBERNETES_TIMEOUT"
)

type K8sClientProvider interface {
	Provide(clientApiConfig *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error)
}

// tokenRotationTransport updates the token of the config when the token is outdated
type tokenRotationTransport struct {
	rt http.RoundTripper
}

// newTokenRotationTransport implements the RoundTripper interface
func newTokenRotationTransport(rt http.RoundTripper) *tokenRotationTransport {
	return &tokenRotationTransport{
		rt: rt,
	}
}

// RoundTrip to wrap http 401 status code in response
func (t *tokenRotationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrK8sUnauthorised
	}
	return resp, err
}

type K8sClient struct{}

func NewK8sClientProvider() *K8sClient {
	return &K8sClient{}
}

func (*K8sClient) Provide(clientApiConfig *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error) {
	return getK8sClientWithApiConfig(clientApiConfig)
}

func (*K8sClient) GetIngressClient() (*ingresses.Client, error) {
	c, _, err := GetK8sClient()
	if err != nil {
		return nil, err
	}
	iClient, err := ingresses.GetClient(c)
	if err != nil {
		return nil, err
	}
	return iClient, nil
}

func GetKubernetesTimeout() time.Duration {
	tOnce.Do(func() {
		timeout = 0 * time.Second
		t, ok := os.LookupEnv(oktetoKubernetesTimeoutEnvVar)
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
	config.WarningHandler = rest.NoWarnings{}

	config.Timeout = GetKubernetesTimeout()

	var client *kubernetes.Clientset

	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return newTokenRotationTransport(rt)
	}

	client, err = kubernetes.NewForConfig(config)
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
	config.WarningHandler = rest.NoWarnings{}

	config.Timeout = GetKubernetesTimeout()

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
	config.WarningHandler = rest.NoWarnings{}

	config.Timeout = GetKubernetesTimeout()

	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return dc, config, err
}
