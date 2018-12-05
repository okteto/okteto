package deployments

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/okteto/cnd/model"
	log "github.com/sirupsen/logrus"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8Yaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

const (
	deploymentFile = "deployment.yml"
)

//DevDeploy deploys a k8 deployment in dev mode
func DevDeploy(dev *model.Dev, namespace string, c *kubernetes.Clientset) (string, error) {
	d, err := loadDeployment(dev, namespace, c)
	if err != nil {
		return "", err
	}

	if d.GetAnnotations()[model.CNDLabel] != "" {
		log.Debugf("The current deployment %s is already in dev mode", GetFullName(d.Namespace, d.Name))
	}

	dev.TurnIntoDevDeployment(d)

	name, err := deploy(d, c)
	if err != nil {
		return "", err
	}

	return name, nil
}

//Deploy deploys a k8 deployment in prod mode
func Deploy(dev *model.Dev, namespace string, c *kubernetes.Clientset) (string, error) {
	prodDeploy, err := loadDeploymentFromFile(dev.Swap.Deployment.File)
	prodDeploy.Namespace = namespace

	if err != nil {
		log.Debugf("error while retrieving deployment from %s: %s", dev.Swap.Deployment.File, err)
		return "", err
	}

	return deploy(prodDeploy, c)
}

func deploy(d *appsv1.Deployment, c *kubernetes.Clientset) (string, error) {
	deploymentName := GetFullName(d.Namespace, d.Name)
	dClient := c.AppsV1().Deployments(d.Namespace)

	if d.Name == "" {
		log.Infof("Creating deployment '%s'...", deploymentName)
		_, err := dClient.Create(d)
		if err != nil {
			return "", fmt.Errorf("Error creating kubernetes deployment: %s", err)
		}
		log.Infof("Created deployment %s.", deploymentName)
	} else {
		log.Infof("Updating deployment '%s'...", deploymentName)
		_, err := dClient.Update(d)
		if err != nil {
			return "", fmt.Errorf("Error updating kubernetes deployment: %s", err)
		}
	}

	log.Infof("Waiting for the deployment '%s' to be ready...", deploymentName)
	tries := 0
	for tries < 60 {
		tries++
		time.Sleep(5 * time.Second)
		d, err := dClient.Get(d.Name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("Error getting kubernetes deployment: %s", err)
		}
		if d.Status.ReadyReplicas == 1 && d.Status.UpdatedReplicas == 1 {
			log.Infof("Kubernetes deployment '%s' is ready.", deploymentName)
			return d.Name, nil
		}
	}
	return "", fmt.Errorf("Kubernetes deployment  %s not ready after 300s", deploymentName)
}

// GetFullName returns the full name of the deployment. This is mostly used for logs and labels
func GetFullName(namespace, deploymentName string) string {
	return fmt.Sprintf("%s/%s", namespace, deploymentName)
}

func containerExists(pod *apiv1.Pod, container string) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name == container {
			return true
		}
	}

	return false
}

// GetCNDPod returns the pod that has the cnd containers
func GetCNDPod(c *kubernetes.Clientset, namespace, deploymentName, devContainer string) (*apiv1.Pod, error) {
	tries := 0
	for tries < 30 {

		pods, err := c.CoreV1().Pods(namespace).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("cnd=%s", deploymentName),
		})

		if err != nil {
			return nil, err
		}

		if len(pods.Items) == 0 {
			return nil, fmt.Errorf("cloud native environment is not initialized. Please run 'cnd up' first")
		}

		pod := pods.Items[0]
		if pod.Status.Phase == apiv1.PodSucceeded || pod.Status.Phase == apiv1.PodFailed {
			return nil, fmt.Errorf("cannot exec in your cloud native environment; current state is %s", pod.Status.Phase)
		}

		var runningPods []apiv1.Pod
		for _, pod := range pods.Items {
			if pod.Status.Phase == apiv1.PodRunning && pod.GetObjectMeta().GetDeletionTimestamp() == nil {
				runningPods = append(runningPods, pod)
			}
		}

		if len(runningPods) == 1 {
			if devContainer != "" {
				if !containerExists(&pod, devContainer) {
					return nil, fmt.Errorf("container %s doesn't exist in the pod", devContainer)
				}
			}

			return &runningPods[0], nil
		}

		if len(runningPods) > 1 {
			podNames := make([]string, len(runningPods))
			for i, p := range runningPods {
				podNames[i] = p.Name
			}
			return nil, fmt.Errorf("more than one cloud native environment have the same name: %+v. Please restart your environment", podNames)
		}

		tries++
		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("kubernetes is taking long to create the dev mode container. Please, check for erros or retry in about 1 minute")
}

func loadDeployment(dev *model.Dev, namespace string, c *kubernetes.Clientset) (*appsv1.Deployment, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	if dev.Swap.Deployment.File != "" {
		log.Infof("loading deployment definition from %s", dev.Swap.Deployment.File)
		return loadDeploymentFromFile(dev.Swap.Deployment.File)
	}

	log.Debugf("loading deployment definition from the cluster")
	return getDeploymentFromAPI(namespace, dev.Swap.Deployment.Name, c)
}

func loadDeploymentFromFile(deploymentPath string) (*appsv1.Deployment, error) {
	file, err := os.Open(deploymentPath)
	if err != nil {
		return nil, err
	}

	dec := k8Yaml.NewYAMLOrJSONDecoder(file, 1000)
	var d appsv1.Deployment
	err = dec.Decode(&d)
	return &d, err
}

func getDeploymentFromAPI(namespace, name string, c *kubernetes.Clientset) (*appsv1.Deployment, error) {

	d, err := c.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		log.Debugf("error while retrieving the deployment: %s", err)
	}

	return d, err
}

func getProdDeploymentPath(namespace, name string) string {
	return path.Join(os.Getenv("HOME"), ".cnd", namespace, name, deploymentFile)

}
