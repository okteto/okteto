package namespaces

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
)

const (
	//OktetoLabel represents the owner of the namespace
	OktetoLabel = "dev.okteto.com"

	// OktetoNotAllowedLabel tells Okteto to not allow operations on the namespace
	OktetoNotAllowedLabel = "dev.okteto.com/not-allowed"
)

var (
	//LimitsSyncthingMemory used by syncthing
	LimitsSyncthingMemory = resource.MustParse("256Mi")
	//LimitsSyncthingCPU used by syncthing
	LimitsSyncthingCPU = resource.MustParse("500m")
)

//IsOktetoNamespace checks if this is a namespace created by okteto
func IsOktetoNamespace(ns *apiv1.Namespace) bool {
	return ns.Labels[OktetoLabel] == "true"
}

//IsOktetoAllowed checks if Okteto operationos are allowed in this namespace
func IsOktetoAllowed(ns *apiv1.Namespace) bool {
	log.Debug("labels: %v", ns.Labels)
	if _, ok := ns.Labels[OktetoNotAllowedLabel]; ok {
		return false
	}

	return true
}

// Get returns the namespace object of ns
func Get(ns string, c *kubernetes.Clientset) (*apiv1.Namespace, error) {
	n, err := c.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
	if err != nil {
		log.Infof("error accessing namespace %s: %s", ns, err)
		return nil, err
	}

	return n, nil
}

//CheckAvailableResources checks if therre are available resources for activating the dev env
func CheckAvailableResources(dev *model.Dev, create bool, d *appsv1.Deployment, c *kubernetes.Clientset) error {
	lr, err := c.CoreV1().LimitRanges(dev.Namespace).Get(dev.Namespace, metav1.GetOptions{})
	if err != nil {
		log.Debugf("cannot query limit ranges: %s", err)
		return nil
	}
	if lr.Spec.Limits == nil || len(lr.Spec.Limits) == 0 {
		log.Debugf("wrong limit range '%s': %s", dev.Namespace, err)
		return nil
	}
	lri := lr.Spec.Limits[0].Default
	rq, err := c.CoreV1().ResourceQuotas(dev.Namespace).Get(dev.Namespace, metav1.GetOptions{})
	if err != nil {
		log.Debugf("cannot query resource quotas: %s", err)
		return nil
	}

	isSyncthingRunning := existsSyncthing(dev, c)

	needCPU, needMemory := getNeededResources(dev, d, create, lri, isSyncthingRunning)
	return checkQuota(needCPU, needMemory, rq)
}

func existsSyncthing(dev *model.Dev, c *kubernetes.Clientset) bool {
	ss, err := c.AppsV1().StatefulSets(dev.Namespace).Get(dev.GetStatefulSetName(), metav1.GetOptions{})
	if err != nil || ss.Name == "" {
		return false
	}
	return true
}

func getNeededResources(dev *model.Dev, d *appsv1.Deployment, create bool, rl apiv1.ResourceList, isSyncthingRunning bool) (resource.Quantity, resource.Quantity) {
	needCPU := resource.MustParse("0")
	needMemory := resource.MustParse("0")
	if !isSyncthingRunning {
		needCPU.Add(LimitsSyncthingCPU)
		needMemory.Add(LimitsSyncthingMemory)
	}
	needCPU.Add(matchCPU(dev, true, 1, d.Spec.Template.Spec, rl))
	needMemory.Add(matchMemory(dev, true, 1, d.Spec.Template.Spec, rl))
	if !create {
		needCPU.Sub(matchCPU(dev, false, d.Status.Replicas, d.Spec.Template.Spec, rl))
		needMemory.Sub(matchMemory(dev, false, d.Status.Replicas, d.Spec.Template.Spec, rl))
	}
	return needCPU, needMemory
}

func checkQuota(needCPU resource.Quantity, needMemory resource.Quantity, rq *apiv1.ResourceQuota) error {
	usedCPU := rq.Status.Used[apiv1.ResourceLimitsCPU]
	totalNeedCPU := needCPU.DeepCopy()
	totalNeedCPU.Add(usedCPU)
	totalCPU := rq.Status.Hard[apiv1.ResourceLimitsCPU]
	if totalNeedCPU.Value() > totalCPU.Value() {
		availableCPU := totalCPU.DeepCopy()
		availableCPU.Sub(usedCPU)
		return fmt.Errorf("Activating your dev environment requires %s cpu and your remaining quota is %s cpu", needCPU.String(), availableCPU.String())
	}
	usedMemory := rq.Status.Used[apiv1.ResourceLimitsMemory]
	totalNeedMemory := needMemory.DeepCopy()
	totalNeedMemory.Add(usedMemory)
	totalMemory := rq.Status.Hard[apiv1.ResourceLimitsMemory]
	if totalNeedMemory.Value() > totalMemory.Value() {
		availableMemory := totalMemory.DeepCopy()
		availableMemory.Sub(usedMemory)
		return fmt.Errorf("Activating your dev environment requires %s of memory and your remaining quota is %s of memory", needMemory.String(), availableMemory.String())
	}
	return nil
}

func matchCPU(dev *model.Dev, isDev bool, replicas int32, spec apiv1.PodSpec, rl apiv1.ResourceList) resource.Quantity {
	cpuPod := resource.MustParse("0")
	for _, c := range spec.Containers {
		v, ok := dev.Resources.Limits[apiv1.ResourceCPU]
		if isDev && ok && c.Name == dev.Container {
			cpuPod.Add(v)
		} else {
			if c.Resources.Limits != nil {
				if v, ok := c.Resources.Limits[apiv1.ResourceCPU]; ok {
					cpuPod.Add(v)
				} else {
					cpuPod.Add(rl[apiv1.ResourceCPU])
				}
			} else {
				cpuPod.Add(rl[apiv1.ResourceCPU])
			}
		}
	}
	cpuTotal := cpuPod
	for i := 1; int32(i) < replicas; i++ {
		cpuTotal.Add(cpuPod)
	}
	return cpuTotal
}

func matchMemory(dev *model.Dev, isDev bool, replicas int32, spec apiv1.PodSpec, rl apiv1.ResourceList) resource.Quantity {
	memoryPod := resource.MustParse("0")
	for _, c := range spec.Containers {
		v, ok := dev.Resources.Limits[apiv1.ResourceMemory]
		if isDev && ok && c.Name == dev.Container {
			memoryPod.Add(v)
		} else {
			if c.Resources.Limits != nil {
				if v, ok := c.Resources.Limits[apiv1.ResourceMemory]; ok {
					memoryPod.Add(v)
				} else {
					memoryPod.Add(rl[apiv1.ResourceMemory])
				}
			} else {
				memoryPod.Add(rl[apiv1.ResourceMemory])
			}
		}
	}
	memoryTotal := memoryPod
	for i := 1; int32(i) < replicas; i++ {
		memoryTotal.Add(memoryPod)
	}
	return memoryTotal
}
