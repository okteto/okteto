package okteto

import (
	"os"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var timeout time.Duration
var tOnce sync.Once

func getKubernetesTimeout() time.Duration {
	tOnce.Do(func() {
		timeout = 0 * time.Second
		t, ok := os.LookupEnv("OKTETO_KUBERNETES_TIMEOUT")
		if !ok {
			return
		}

		parsed, err := time.ParseDuration(t)
		if err != nil {
			log.Infof("'%s' is not a valid duration, ignoring", t)
			return
		}

		log.Infof("OKTETO_KUBERNETES_TIMEOUT applied: '%s'", parsed.String())
		timeout = parsed
	})

	return timeout
}

func getK8sClient(clientApiConfig *clientcmdapi.Config) (*kubernetes.Clientset, *rest.Config, error) {
	clientConfig := clientcmd.NewDefaultClientConfig(*clientApiConfig, nil)

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	config.Timeout = getKubernetesTimeout()

	Context().SetClusterType(config.Host)
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return client, config, nil
}
