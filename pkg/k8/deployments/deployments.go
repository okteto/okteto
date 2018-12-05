package deployments

import (
	"fmt"
	"os"
	"time"

	"github.com/okteto/cnd/pkg/k8/util"
	"github.com/okteto/cnd/pkg/model"
	log "github.com/sirupsen/logrus"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8Yaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

//DevDeploy deploys a k8 deployment in dev mode
func DevDeploy(dev *model.Dev, namespace string, c *kubernetes.Clientset) (string, error) {
	var d *appsv1.Deployment
	var err error
	if dev.Swap.Deployment.File != "" {
		log.Infof("loading deployment definition from %s", dev.Swap.Deployment.File)
		d, err = loadDeploymentFromFile(dev.Swap.Deployment.File)
	} else {
		d, err = loadDeployment(namespace, dev.Swap.Deployment.Name, c)
	}

	if err != nil {
		return "", err
	}

	parentRevision := d.GetObjectMeta().GetAnnotations()[model.RevisionAnnotation]
	if d.GetObjectMeta().GetLabels()[model.CNDLabel] != "" {
		log.Debugf("The current deployment %s is already in dev mode. Leaving the original parent revision.", GetFullName(d.Namespace, d.Name))
		parentRevision = d.GetObjectMeta().GetAnnotations()[model.CNDRevisionAnnotation]
	}

	dev.TurnIntoDevDeployment(d, parentRevision)

	name, err := deploy(d, c)
	if err != nil {
		return "", err
	}

	return name, nil
}

//Deploy deploys a k8 deployment in prod mode
func Deploy(dev *model.Dev, namespace string, c *kubernetes.Clientset) (string, error) {
	if dev.Swap.Deployment.File != "" {
		prodDeploy, err := loadDeploymentFromFile(dev.Swap.Deployment.File)
		prodDeploy.Namespace = namespace

		if err != nil {
			log.Debugf("error while retrieving deployment from %s: %s", dev.Swap.Deployment.File, err)
			return "", err
		}

		return deploy(prodDeploy, c)
	}

	d, err := loadDeployment(namespace, dev.Swap.Deployment.Name, c)
	if err != nil {
		return "", err
	}

	fullname := GetFullName(d.Namespace, d.Name)

	revision := d.GetObjectMeta().GetAnnotations()[model.CNDRevisionAnnotation]
	if revision == "" {
		log.Debugf("%s doesn't have the %s annotation", fullname, model.CNDRevisionAnnotation)
		return "", fmt.Errorf("%s is not in dev mode", fullname)
	}

	rs, err := getMatchingReplicaSet(namespace, d.Name, revision, c)
	if err != nil {
		return "", err
	}

	util.SetFromReplicaSetTemplate(d, rs.Spec.Template)
	util.SetDeploymentAnnotationsTo(d, rs)

	delete(d.GetObjectMeta().GetAnnotations(), model.CNDRevisionAnnotation)
	delete(d.GetObjectMeta().GetLabels(), model.CNDLabel)

	log.Infof("restoring the production configuration")
	return deploy(d, c)
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
			LabelSelector: fmt.Sprintf("%s=%s", model.CNDLabel, deploymentName),
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

func loadDeployment(namespace, deploymentName string, c *kubernetes.Clientset) (*appsv1.Deployment, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	d, err := c.AppsV1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Debugf("error while retrieving the deployment: %s", err)
	}

	return d, err
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

func getMatchingReplicaSet(namespace, deploymentName, revision string, c *kubernetes.Clientset) (*appsv1.ReplicaSet, error) {
	log.Debugf("Looking for a replica set of %s/%s with revision %s", namespace, deploymentName, revision)

	// TODO: how can we add a filter so we only get the specific one we need?
	replicaSets, err := c.AppsV1().ReplicaSets(namespace).List(metav1.ListOptions{})
	if err != nil {
		log.Debugf("error while retrieving replicasets: %s", err)
		return nil, err
	}

	log.Debugf("found %d replica sets on namespace %s", len(replicaSets.Items), namespace)

	var matchingReplicaSet *appsv1.ReplicaSet

	for _, r := range replicaSets.Items {
		ownerReferences := r.GetObjectMeta().GetOwnerReferences()
		if len(ownerReferences) == 0 {
			log.Errorf("replicaset %s doesn't have an owner reference", r.Name)
			continue
		}

		name := r.GetObjectMeta().GetOwnerReferences()[0].Name
		if name == "" {
			log.Errorf("replicaset %s doesn't have an owner name", r.Name)
			continue
		}

		replicaSetRevision := r.GetObjectMeta().GetAnnotations()[model.RevisionAnnotation]
		if replicaSetRevision == "" {
			log.Errorf("replicaset %s doesn't have a revision", r.Name)
			continue
		}

		if name == deploymentName {
			if replicaSetRevision == revision {
				matchingReplicaSet = &r
				log.Debugf("replicaset %s has the required revision", r.Name)
				break
			}
		}
	}

	if matchingReplicaSet == nil {
		return nil, fmt.Errorf("couldn't find a replicaset of %s with the required revision", deploymentName)
	}

	return matchingReplicaSet, nil
}
