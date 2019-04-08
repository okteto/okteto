package deployments

import (
	"context"
	"fmt"
	"strings"
	"time"

	"encoding/json"

	"cli/cnd/pkg/k8/secrets"
	"cli/cnd/pkg/log"
	"cli/cnd/pkg/model"

	"cli/cnd/pkg/k8/volume"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	maxRetries = 300
)

//Get returns a deployment object given its name and namespace
func Get(namespace, deployment string, c *kubernetes.Clientset) (*appsv1.Deployment, error) {
	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}
	d, err := c.AppsV1().Deployments(namespace).Get(deployment, metav1.GetOptions{})
	if err != nil {
		log.Debugf("error while retrieving the deployment %s: %s", GetFullName(namespace, deployment), err)
		return nil, err
	}

	return d, nil
}

//DevModeOn activates a cloud native development for a given k8 deployment
func DevModeOn(d *appsv1.Deployment, devList []*model.Dev, c *kubernetes.Clientset) error {
	manifest := getAnnotation(d.GetObjectMeta(), model.CNDDeploymentAnnotation)
	if manifest != "" {
		dOrig := &appsv1.Deployment{}
		if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
			return err
		}
		dOrig.ResourceVersion = ""
		annotations := dOrig.GetObjectMeta().GetAnnotations()
		delete(annotations, revisionAnnotation)
		dOrig.GetObjectMeta().SetAnnotations(annotations)
		d = dOrig
	}

	if err := translateToDevModeDeployment(d, devList); err != nil {
		return err
	}

	if err := secrets.Create(d, devList, c); err != nil {
		return err
	}

	if err := volume.Deploy(d, devList, c); err != nil {
		return err
	}

	if err := Update(d, c); err != nil {
		return err
	}

	return nil
}

//DevModeOff deactivates a cloud native development
func DevModeOff(d *appsv1.Deployment, dev *model.Dev, c *kubernetes.Clientset, removeVolumes bool) error {
	manifest := getAnnotation(d.GetObjectMeta(), model.CNDDeploymentAnnotation)
	if manifest != "" {
		dOrig := &appsv1.Deployment{}
		if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
			return err
		}
		dOrig.ResourceVersion = ""
		log.Infof("restoring the production configuration")
		if err := Update(dOrig, c); err != nil {
			return err
		}
	} else {
		fullname := GetFullName(d.Namespace, d.Name)
		log.Debugf("%s doesn't have the %s annotation", fullname, model.CNDDeploymentAnnotation)
	}

	log.Infof("deleting syncthing secret")
	if err := secrets.Delete(d, c); err != nil {
		return err
	}

	if removeVolumes {
		log.Infof("deleting persistent volumes")
		if err := volume.Destroy(dev.GetCNDSyncVolume(), d.Namespace, c); err != nil {
			return err
		}
		for _, v := range dev.Volumes {
			if err := volume.Destroy(dev.GetCNDDataVolume(v), d.Namespace, c); err != nil {
				return err
			}
		}
		if dev.EnableDocker {
			if err := volume.Destroy(dev.GetCNDDinDVolume(), d.Namespace, c); err != nil {
				return err
			}
		}
	}

	return nil
}

// Create creates a deployment
func Create(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	deploymentName := GetFullName(d.Namespace, d.Name)
	log.Infof("creating deployment '%s'...", deploymentName)
	dClient := c.AppsV1().Deployments(d.Namespace)
	_, err := dClient.Create(d)
	if err != nil {
		return fmt.Errorf("Error creating kubernetes deployment: %s", err)
	}
	log.Infof("deployment %s created", deploymentName)
	return nil
}

// Update updates a deployment
func Update(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	deploymentName := GetFullName(d.Namespace, d.Name)
	log.Infof("updating deployment '%s'...", deploymentName)
	dClient := c.AppsV1().Deployments(d.Namespace)
	pods, err := c.CoreV1().Pods(d.Namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", model.CNDLabel, d.Name),
	})
	if err != nil {
		return fmt.Errorf("Error querying pods: %s", err)
	}
	dPrev, err := Get(d.Namespace, d.Name, c)
	if err != nil {
		return fmt.Errorf("Error getting kubernetes deployment: %s", err)
	}
	revision := getAnnotation(dPrev.GetObjectMeta(), revisionAnnotation)
	_, err = dClient.Update(d)
	if err != nil {
		return fmt.Errorf("Error updating kubernetes deployment: %s", err)
	}
	newRevision := ""
	for newRevision == "" {
		time.Sleep(1 * time.Second)
		dPost, err := Get(d.Namespace, d.Name, c)
		if err != nil {
			return fmt.Errorf("Error getting kubernetes deployment: %s", err)
		}
		newRevision = getAnnotation(dPost.GetObjectMeta(), revisionAnnotation)
	}
	if newRevision != revision {
		log.Infof("deployment '%s' updated", deploymentName)
		waitForOldPodsToScheduleForTermination(d, pods.Items, c)
	} else {
		log.Debugf("deployment '%s' not updated", deploymentName)
	}
	return nil
}

// Destroy destroys a deployment
func Destroy(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	deploymentName := GetFullName(d.Namespace, d.Name)
	log.Infof("destroying deployment '%s'...", deploymentName)
	dClient := c.AppsV1().Deployments(d.Namespace)
	if err := dClient.Delete(d.Name, &metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds}); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("couldn't destroy your cloud native desk: %s", err)
		}
	}
	log.Infof("deployment '%s' destroyed", deploymentName)
	return nil
}

func waitForOldPodsToScheduleForTermination(d *appsv1.Deployment, oldPods []apiv1.Pod, c *kubernetes.Clientset) error {
	updated := false
	tries := 0
	for !updated && tries < 60 {
		newPods, err := c.CoreV1().Pods(d.Namespace).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", model.CNDLabel, d.Name),
		})
		if err != nil {
			return fmt.Errorf("Error querying pods: %s", err)
		}
		updated = true
		for _, oldPod := range oldPods {
			for _, newPod := range newPods.Items {
				if newPod.Name == oldPod.Name {
					if newPod.Status.Phase == apiv1.PodRunning && newPod.GetObjectMeta().GetDeletionTimestamp() == nil {
						log.Debugf("Old Pod is still running '%s'...", newPod.Name)
						updated = false
						break
					}
				}
			}
			if !updated {
				time.Sleep(1 * time.Second)
				break
			}
		}
		tries++
	}
	return nil
}

func getPodEventChannel(namespace string, field fields.Set, c *kubernetes.Clientset) (<-chan watch.Event, error) {
	w, err := c.Core().Events(namespace).Watch(
		metav1.ListOptions{
			FieldSelector: field.AsSelector().String(),
		},
	)
	if err != nil {
		log.Info(err)
		return nil, err
	}
	ch := w.ResultChan()
	return ch, nil
}

//GetPodEvents shows the events of a given pod
func GetPodEvents(ctx context.Context, pod *apiv1.Pod, c *kubernetes.Clientset) {
	log.Debugf("Start monitoring events for pod-%s", pod.Name)
	field := fields.Set{}
	field["involvedObject.uid"] = string(pod.GetUID())
	ch, err := getPodEventChannel(pod.Namespace, field, c)
	if err != nil {
		return
	}

	for {
		select {
		case e := <-ch:
			if e.Object == nil {
				log.Infof("received an nil event.Object from the pod event call: %+v", e)
				time.Sleep(1 * time.Second)
				ch, err = getPodEventChannel(pod.Namespace, field, c)
				if err != nil {
					return
				}
				continue
			}

			event, ok := e.Object.(*apiv1.Event)
			if !ok {
				log.Infof("couldn't convert e.Object to apiv1.Event: %+v", e)
				time.Sleep(1 * time.Second)
				continue
			}

			if event.Type == "Normal" {
				log.Debugf("kubernetes: %s", event.Message)
			} else {
				log.Red("Kubernetes: %s", event.Message)
			}

		case <-ctx.Done():
			log.Debug("pod events shutdown")
			return
		}
	}
}

// GetCNDPod returns the pod that has the cnd containers
func GetCNDPod(ctx context.Context, namespace, name string, c *kubernetes.Clientset) (*apiv1.Pod, error) {
	tries := 0
	ticker := time.NewTicker(1 * time.Second)

	for tries < maxRetries {
		log.Debugf("getting cnd pod-%s", name)
		pods, err := c.CoreV1().Pods(namespace).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", model.CNDLabel, name),
		})

		if err != nil {
			return nil, err
		}

		var pendingOrRunningPods []apiv1.Pod
		for _, pod := range pods.Items {
			if pod.Status.Phase == apiv1.PodRunning {
				if pod.GetObjectMeta().GetDeletionTimestamp() == nil {
					pendingOrRunningPods = append(pendingOrRunningPods, pod)
				}
			} else {
				log.Debugf("cnd pod %s/%s is on %s, waiting for it to be running", pod.Namespace, pod.Name, pod.Status.Phase)
			}
		}

		if len(pendingOrRunningPods) == 1 {
			log.Debugf("cnd %s/pod/%s is %s", pendingOrRunningPods[0].Namespace, pendingOrRunningPods[0].Name, pendingOrRunningPods[0].Status.Phase)
			return &pendingOrRunningPods[0], nil
		}

		if len(pendingOrRunningPods) > 1 {
			podNames := make([]string, len(pendingOrRunningPods))
			for i, p := range pendingOrRunningPods {
				podNames[i] = p.Name
			}
			return nil, fmt.Errorf("more than one cloud native environment have the same name: %+v. Please restart your environment", podNames)
		}

		select {
		case <-ticker.C:
			tries++
			continue
		case <-ctx.Done():
			log.Debug("cancelling call to get cnd pod")
			return nil, ctx.Err()
		}
	}

	log.Debugf("cnd pod wasn't running after %d seconds", maxRetries)
	return nil, fmt.Errorf("kubernetes is taking too long to create the cloud native environment. Please check for errors and try again")
}
