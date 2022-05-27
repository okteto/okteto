package integration

import (
	"context"

	"github.com/okteto/okteto/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// K8sClient returns a kubernetes client for current KUBECONFIG
func K8sClient() (*kubernetes.Clientset, *rest.Config, error) {
	clientConfig := getClientConfig(config.GetKubeconfigPath(), "")

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return client, config, nil
}

func getClientConfig(kubeconfigPaths []string, kubeContext string) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	loadingRules.Precedence = kubeconfigPaths
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{
			CurrentContext: kubeContext,
			ClusterInfo:    clientcmdapi.Cluster{Server: ""},
		},
	)
}

// GetDeployment returns a deployment given a namespace and name
func GetDeployment(ctx context.Context, ns, name string) (*appsv1.Deployment, error) {
	client, _, err := K8sClient()
	if err != nil {
		return nil, err
	}

	return client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
}
