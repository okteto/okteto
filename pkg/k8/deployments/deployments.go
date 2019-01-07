package deployments

import (
	"fmt"
	"time"

	"encoding/json"

	"github.com/okteto/cnd/pkg/k8/cp"
	"github.com/okteto/cnd/pkg/model"
	log "github.com/sirupsen/logrus"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

//Get returns a deployment object given its name and namespace
func Get(namespace, deploymentName string, c *kubernetes.Clientset) (*appsv1.Deployment, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	d, err := c.AppsV1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Debugf("error while retrieving the deployment: %s", err)
	}

	return d, err
}

//DevModeOn activates a cloud native development for a given k8 deployment
func DevModeOn(dev *model.Dev, d *appsv1.Deployment, c *kubernetes.Clientset) error {
	dev.Swap.Deployment.Container = getDevContainerOrFirst(dev.Swap.Deployment.Container, d.Spec.Template.Spec.Containers)

	manifest := getAnnotation(d.GetObjectMeta(), model.CNDDeploymentAnnotation)
	if manifest != "" {
		dOrig := &appsv1.Deployment{}
		if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
			return err
		}
		dOrig.ResourceVersion = ""
		d = dOrig
	}

	if err := translateToDevModeDeployment(d, dev); err != nil {
		return err
	}

	if err := deploy(d, c); err != nil {
		return err
	}

	return nil
}

//DevModeOff deactivates a cloud native development
func DevModeOff(dev *model.Dev, d *appsv1.Deployment, c *kubernetes.Clientset) error {
	manifest := getAnnotation(d.GetObjectMeta(), model.CNDDeploymentAnnotation)
	if manifest == "" {
		fullname := GetFullName(d.Namespace, d.Name)
		log.Debugf("%s doesn't have the %s annotation", fullname, model.CNDDeploymentAnnotation)
		return nil
	}

	dOrig := &appsv1.Deployment{}
	if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
		return err
	}
	dOrig.ResourceVersion = ""

	log.Infof("restoring the production configuration")
	return deploy(dOrig, c)
}

func deploy(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	deploymentName := GetFullName(d.Namespace, d.Name)
	dClient := c.AppsV1().Deployments(d.Namespace)

	if d.Name == "" {
		log.Infof("Creating deployment '%s'...", deploymentName)
		_, err := dClient.Create(d)
		if err != nil {
			return fmt.Errorf("Error creating kubernetes deployment: %s", err)
		}
		log.Infof("Created deployment %s", deploymentName)
	} else {
		log.Infof("Updating deployment '%s'...", deploymentName)
		_, err := dClient.Update(d)
		if err != nil {
			return fmt.Errorf("Error updating kubernetes deployment: %s", err)
		}
	}

	return nil
}

// GetCNDPod returns the pod that has the cnd containers
func GetCNDPod(d *appsv1.Deployment, c *kubernetes.Clientset) (*apiv1.Pod, error) {
	tries := 0
	for tries < 30 {

		pods, err := c.CoreV1().Pods(d.Namespace).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", model.CNDLabel, d.Name),
		})

		if err != nil {
			return nil, err
		}

		var pendingOrRunningPods []apiv1.Pod
		for _, pod := range pods.Items {
			if pod.Status.Phase == apiv1.PodRunning || pod.Status.Phase == apiv1.PodPending {
				if pod.GetObjectMeta().GetDeletionTimestamp() == nil {
					pendingOrRunningPods = append(pendingOrRunningPods, pod)
				}
			}
		}

		if len(pendingOrRunningPods) == 1 {
			return &pendingOrRunningPods[0], nil
		}

		if len(pendingOrRunningPods) > 1 {
			podNames := make([]string, len(pendingOrRunningPods))
			for i, p := range pendingOrRunningPods {
				podNames[i] = p.Name
			}
			return nil, fmt.Errorf("more than one cloud native environment have the same name: %+v. Please restart your environment", podNames)
		}

		tries++
		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("kubernetes is taking too long to create the cloud native environment. Please check for errors or try again")
}

// InitVolumeWithTarball initializes the remote volume with a local tarball
func InitVolumeWithTarball(c *kubernetes.Clientset, config *rest.Config, namespace, podName, folder string) error {
	copied := false
	tries := 0
	for tries < 30 && !copied {
		pod, err := c.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for _, status := range pod.Status.InitContainerStatuses {
			if status.Name == model.CNDInitSyncContainerName {
				if status.State.Waiting != nil {
					time.Sleep(1 * time.Second)
				}
				if status.State.Running != nil {
					if copied {
						time.Sleep(1 * time.Second)
					} else {
						if err := cp.Copy(c, config, namespace, pod, folder); err != nil {
							return err
						}
						copied = true
					}
				}
				if status.State.Terminated != nil {
					if status.State.Terminated.ExitCode != 0 {
						return fmt.Errorf("Volume initialization failed with exit code %d", status.State.Terminated.ExitCode)
					}
					copied = true
				}
				break
			}
		}
		tries++
	}
	if tries == 30 {
		return fmt.Errorf("kubernetes is taking too long to create the cloud native environment. Please check for errors or try again")
	}
	tries = 0
	for tries < 30 {
		pod, err := c.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pod.Status.Phase == apiv1.PodRunning {
			return nil
		}
		tries++
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("kubernetes is taking too long to create the cloud native environment. Please check for errors or try again")
}
