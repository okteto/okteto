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

package okteto

import (
	"errors"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/k8s/ingresses"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	ioCtrl "github.com/okteto/okteto/pkg/log/io"
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

type K8sClientProviderWithLogger interface {
	ProvideWithLogger(clientApiConfig *clientcmdapi.Config, k8sLogger *ioCtrl.K8sLogger) (kubernetes.Interface, *rest.Config, error)
}

// tokenRotationTransport updates the token of the config when the token is outdated
type tokenRotationTransport struct {
	rt        http.RoundTripper
	k8sLogger *ioCtrl.K8sLogger
}

// newTokenRotationTransport implements the RoundTripper interface
func newTokenRotationTransport(rt http.RoundTripper, k8sLogger *ioCtrl.K8sLogger) *tokenRotationTransport {
	return &tokenRotationTransport{
		rt:        rt,
		k8sLogger: k8sLogger,
	}
}

// RoundTrip to wrap http 401 status code in response
func (t *tokenRotationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.rt.RoundTrip(req)
	if t.k8sLogger != nil && t.k8sLogger.IsEnabled() {
		var statusCode int
		if resp != nil {
			statusCode = resp.StatusCode
		}
		t.k8sLogger.Log(statusCode, req.Method, req.URL.String())
	}
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrK8sUnauthorised
	}
	return resp, err
}

type K8sClient struct {
	oktetoK8sLogger *ioCtrl.K8sLogger
}

// NewK8sClientProvider returns new K8sClient
func NewK8sClientProvider() *K8sClient {
	return &K8sClient{}
}

// NewK8sClientProviderWithLogger returns new K8sClient that logs all requests to k8s
func NewK8sClientProviderWithLogger(oktetoK8sLogger *ioCtrl.K8sLogger) *K8sClient {
	return &K8sClient{
		oktetoK8sLogger: oktetoK8sLogger,
	}
}

func (k *K8sClient) Provide(clientApiConfig *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error) {
	return getK8sClientWithApiConfig(clientApiConfig, k.oktetoK8sLogger)
}

func (*K8sClient) ProvideWithLogger(clientApiConfig *clientcmdapi.Config, k8sLogger *ioCtrl.K8sLogger) (kubernetes.Interface, *rest.Config, error) {
	return getK8sClientWithApiConfig(clientApiConfig, k8sLogger)
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

func getK8sClientWithApiConfig(clientApiConfig *clientcmdapi.Config, k8sLogger *ioCtrl.K8sLogger) (*kubernetes.Clientset, *rest.Config, error) {
	clientConfig := clientcmd.NewDefaultClientConfig(*clientApiConfig, nil)
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	config.WarningHandler = rest.NoWarnings{}

	config.Timeout = GetKubernetesTimeout()

	var client *kubernetes.Clientset

	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return newTokenRotationTransport(rt, k8sLogger)
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
