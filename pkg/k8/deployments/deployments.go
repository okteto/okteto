package deployments

import (
	"context"
	"fmt"
	"time"

	"encoding/json"

	. "github.com/logrusorgru/aurora"
	"github.com/cloudnativedevelopment/cnd/pkg/k8/cp"
	"github.com/cloudnativedevelopment/cnd/pkg/k8/secrets"
	"github.com/cloudnativedevelopment/cnd/pkg/model"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
		d = dOrig
	}

	if err := translateToDevModeDeployment(d, devList); err != nil {
		return err
	}

	if err := secrets.Create(d, devList, c); err != nil {
		return err
	}

	if err := Deploy(d, c); err != nil {
		return err
	}

	return nil
}

//DevModeOff deactivates a cloud native development
func DevModeOff(d *appsv1.Deployment, c *kubernetes.Clientset) error {
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
	if err := Deploy(dOrig, c); err != nil {
		return err
	}

	log.Infof("deleting syncthing secret")
	if err := secrets.Delete(d, c); err != nil {
		return err
	}

	return nil
}

// Deploy deploys or updates d
func Deploy(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	deploymentName := GetFullName(d.Namespace, d.Name)
	dClient := c.AppsV1().Deployments(d.Namespace)

	if d.Name == "" {
		log.Infof("Creating deployment on '%s'...", d.Namespace)
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
		log.Debugf("Updated deployment '%s'...", deploymentName)
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
		log.Error(err)
		return nil, err
	}
	ch := w.ResultChan()
	return ch, nil
}

//GetPodEvents shows the events of a given pod
func GetPodEvents(ctx context.Context, pod *apiv1.Pod, c *kubernetes.Clientset) {
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
				log.Errorf("couldn't convert e.Object to apiv1.Event: %+v", e)
				time.Sleep(1 * time.Second)
				continue
			}

			if event.Type == "Normal" {
				log.Debug(event.Message)
			} else {
				fmt.Println(Red("Kubernetes: "), event.Message)
			}

		case <-ctx.Done():
			log.Debug("pod events shutdown")
			return
		}
	}
}

// GetCNDPod returns the pod that has the cnd containers
func GetCNDPod(ctx context.Context, d *appsv1.Deployment, c *kubernetes.Clientset) (*apiv1.Pod, error) {
	tries := 0
	ticker := time.NewTicker(1 * time.Second)

	log.Debugf("Waiting for cnd pod to be ready")
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
			} else {
				log.Debugf("cnd pod is on %s, waiting", pod.Status.String())
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

		select {
		case <-ticker.C:
			tries++
			continue
		case <-ctx.Done():
			log.Debug("cancelling call to get cnd pod")
			return nil, ctx.Err()
		}
	}

	log.Debugf("cnd pod wasn't running after 30 seconds")
	return nil, fmt.Errorf("kubernetes is taking too long to create the cloud native environment. Please check for errors and try again")
}

func waitForInitToBeReady(ctx context.Context, c *kubernetes.Clientset, config *rest.Config, namespace, podName string, devList []*model.Dev) error {
	ticker := time.NewTicker(1 * time.Second)
	for _, dev := range devList {
		copied := false
		tries := 0

		for tries < 30 && !copied {
			pod, err := c.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			for _, status := range pod.Status.InitContainerStatuses {
				if status.Name == dev.GetCNDInitSyncContainer() {
					if status.State.Waiting != nil {
						time.Sleep(1 * time.Second)
					}
					if status.State.Running != nil {
						log.Debugf("cnd-sync init cointainer is now running, sending the tarball")
						if copied {
							time.Sleep(1 * time.Second)
						} else {
							if err := cp.Copy(c, config, namespace, pod, dev); err != nil {
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

			select {
			case <-ticker.C:
				tries++
				continue
			case <-ctx.Done():
				log.Debug("cancelling call to get cnd pod")
				return ctx.Err()
			}
		}
		if tries == 30 {
			log.Debugf("cnd-sync didn't finish copying the tarball after 30 seconds")
			return fmt.Errorf("kubernetes is taking too long to create the cloud native environment. Please check for errors and try again")
		}
	}

	return nil
}

func waitForDevPodToBeRunning(ctx context.Context, c *kubernetes.Clientset, namespace, podName string) error {
	ticker := time.NewTicker(1 * time.Second)

	tries := 0
	log.Debugf("waiting for dev container to start running")
	for tries < 30 {
		pod, err := c.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pod.Status.Phase == apiv1.PodRunning {
			return nil
		}

		select {
		case <-ticker.C:
			tries++
			continue
		case <-ctx.Done():
			log.Debug("cancelling call to get cnd pod")
			return ctx.Err()
		}
	}

	log.Debugf("dev container didn't start running after 30 seconds")
	return fmt.Errorf("kubernetes is taking too long to create the cloud native environment. Please check for errors and try again")
}

// InitVolumeWithTarball initializes the remote volume with a local tarball
func InitVolumeWithTarball(ctx context.Context, c *kubernetes.Clientset, config *rest.Config, namespace, podName string, devList []*model.Dev) error {

	if err := waitForInitToBeReady(ctx, c, config, namespace, podName, devList); err != nil {
		return err
	}

	return waitForDevPodToBeRunning(ctx, c, namespace, podName)
}
