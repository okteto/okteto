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

package integration

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

func GetPodsBySelector(ctx context.Context, ns, selector string, client kubernetes.Interface) (*corev1.PodList, error) {
	return client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
}

// GetService returns a service given a namespace and a name
func GetService(ctx context.Context, ns, name string, client kubernetes.Interface) (*corev1.Service, error) {
	return client.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
}

// GetDeployment returns a deployment given a namespace and name
func GetDeployment(ctx context.Context, ns, name string, client kubernetes.Interface) (*appsv1.Deployment, error) {
	return client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
}

// UpdateDeployment updates a current deployment
func UpdateDeployment(ctx context.Context, ns string, d *appsv1.Deployment, client kubernetes.Interface) error {
	_, err := client.AppsV1().Deployments(ns).Update(ctx, d, metav1.UpdateOptions{})
	return err
}

// UpdateStatefulset updates a statefulset
func UpdateStatefulset(ctx context.Context, ns string, sfs *appsv1.StatefulSet, client kubernetes.Interface) error {
	_, err := client.AppsV1().StatefulSets(ns).Update(ctx, sfs, metav1.UpdateOptions{})
	return err
}

// GetConfigmap returns a configmap given name and namespace
func GetConfigmap(ctx context.Context, ns, name string, client kubernetes.Interface) (*corev1.ConfigMap, error) {
	return client.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
}

// GetDeploymentList returns all deployments given a namespace
func GetDeploymentList(ctx context.Context, ns string, client kubernetes.Interface) ([]appsv1.Deployment, error) {
	dList, err := client.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	return dList.Items, err
}

// GetStatefulset returns a sfs given a namespace and name
func GetStatefulset(ctx context.Context, ns, name string, client kubernetes.Interface) (*appsv1.StatefulSet, error) {
	return client.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
}

// GetStatefulsetList returns all sfs given a namespace
func GetStatefulsetList(ctx context.Context, ns string, client kubernetes.Interface) ([]appsv1.StatefulSet, error) {
	sfsList, err := client.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
	return sfsList.Items, err
}

// WaitForDeployment waits until a deployment is rollout correctly
func WaitForDeployment(kubectlBinary string, kubectlOpts *commands.KubectlOptions, revision int, timeout time.Duration) error {
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	retry := 0
	for {
		select {
		case <-to.C:
			return fmt.Errorf("%s didn't rollout after %s", kubectlOpts.Name, timeout.String())
		case <-ticker.C:
			output, err := commands.RunKubectlRolloutDeployment(kubectlBinary, kubectlOpts, revision)
			if err != nil {
				continue
			}

			if retry%10 == 0 {
				log.Printf("waitForDeployment output: %s", output)
			}

			if strings.Contains(output, "is different from the running revision") {
				r := regexp.MustCompile(`\(\d+\)`)
				matches := r.FindAllString(output, -1)
				validMatchesLength := 2
				if len(matches) == validMatchesLength {
					desiredVersion := strings.ReplaceAll(strings.ReplaceAll(matches[0], "(", ""), ")", "")
					runningVersion := strings.ReplaceAll(strings.ReplaceAll(matches[0], "(", ""), ")", "")
					if desiredVersion <= runningVersion {
						log.Println(output)
						return nil
					}
				}
				continue
			}
			if strings.Contains(output, "successfully rolled out") {
				log.Println(output)
				return nil
			}
		}
	}
}

// WaitForStatefulset waits until a sfs is rollout correctly
func WaitForStatefulset(kubectlBinary string, kubectlOpts *commands.KubectlOptions, timeout time.Duration) error {
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	for {
		select {
		case <-to.C:
			return fmt.Errorf("%s didn't rollout after %s", kubectlOpts, timeout.String())
		case <-ticker.C:
			output, err := commands.RunKubectlRolloutStatefulset(kubectlBinary, kubectlOpts)
			if err != nil {
				continue
			}
			log.Printf("waitForStatefulset output: %s", output)

			if strings.Contains(output, "is different from the running revision") {
				continue
			}

			if strings.Contains(output, "partitioned roll out complete") || strings.Contains(output, "rolling update complete") {
				log.Println(output)
				return nil
			}
		}
	}
}

// DestroyPod returns a deployment given a namespace and name
func DestroyPod(ctx context.Context, ns, labelSelector string, c kubernetes.Interface) error {
	log.Printf("destroying pods with label selector: %s", labelSelector)

	pods, err := c.CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("failed to retrieve deployment %s pods: %w", labelSelector, err)
	}
	var zero int64 = 0
	if len(pods.Items) == 0 {
		return fmt.Errorf("not detected any pod")
	}
	for idx := range pods.Items {
		log.Printf("destroying pod %s", pods.Items[idx].Name)
		err := c.CoreV1().Pods(ns).Delete(
			ctx,
			pods.Items[idx].Name,
			metav1.DeleteOptions{GracePeriodSeconds: &zero},
		)
		if err != nil {
			return fmt.Errorf("error deleting pod %s: %w", pods.Items[idx].Name, err)
		}
	}

	return nil
}
