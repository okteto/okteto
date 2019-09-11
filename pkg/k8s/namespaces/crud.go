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
)

//IsOktetoNamespace checks if this is a namespace created by okteto
func IsOktetoNamespace(ns string, c *kubernetes.Clientset) bool {
	n, err := c.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
	if err != nil {
		log.Infof("error accessing namespace %s: %s", ns, err)
		return false
	}
	return n.Labels[OktetoLabel] == "true"
}

//CheckAvailableResources checks if therre are available resources for activating the dev env
func CheckAvailableResources(dev *model.Dev, create bool, d *appsv1.Deployment, c *kubernetes.Clientset) error {
	needCPU := resource.MustParse("0")
	needMemory := resource.MustParse("0")
	if !existsSyncthing(dev, c) {
		needCPU.Add(resource.MustParse("500m"))
		needMemory.Add(resource.MustParse("256Mi"))
	}
	lr, err := c.CoreV1().LimitRanges(dev.Namespace).Get(dev.Namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}
	needCPU.Add(matchCPU(dev, true, 1, d.Spec.Template.Spec, lr))
	needMemory.Add(matchMemory(dev, true, 1, d.Spec.Template.Spec, lr))
	if !create {
		needCPU.Sub(matchCPU(dev, false, d.Status.Replicas, d.Spec.Template.Spec, lr))
		needMemory.Sub(matchMemory(dev, false, d.Status.Replicas, d.Spec.Template.Spec, lr))
	}
	rq, err := c.CoreV1().ResourceQuotas(dev.Namespace).Get(dev.Namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}
	usedCPU := rq.Status.Used[apiv1.ResourceLimitsCPU]
	usedCPU.Add(needCPU)
	totalCPU := rq.Status.Hard[apiv1.ResourceLimitsCPU]
	if usedCPU.Value() > totalCPU.Value() {
		usedCPU.Sub(needCPU)
		totalCPU.Sub(usedCPU)
		return fmt.Errorf("Activating your dev environment requires %s cpu and your remaining quota is %s cpu", needCPU.String(), totalCPU.String())
	}
	usedMemory := rq.Status.Used[apiv1.ResourceLimitsMemory]
	usedMemory.Add(needMemory)
	totalMemory := rq.Status.Hard[apiv1.ResourceLimitsMemory]
	if usedMemory.Value() > totalMemory.Value() {
		usedMemory.Sub(needMemory)
		totalMemory.Sub(usedMemory)
		return fmt.Errorf("Activating your dev environment requires %s of memory and your remaining quota is %s of memory", needMemory.String(), totalMemory.String())
	}
	return nil
}

func existsSyncthing(dev *model.Dev, c *kubernetes.Clientset) bool {
	ss, err := c.AppsV1().StatefulSets(dev.Namespace).Get(dev.GetStatefulSetName(), metav1.GetOptions{})
	if err != nil || ss.Name == "" {
		return false
	}
	return true
}

func matchCPU(dev *model.Dev, isDev bool, replicas int32, spec apiv1.PodSpec, lr *apiv1.LimitRange) resource.Quantity {
	cpuPod := resource.MustParse("0")
	for _, c := range spec.Containers {
		v, ok := dev.Resources.Limits[apiv1.ResourceCPU]
		if !isDev || !ok {
			if c.Resources.Limits == nil {
				cpuPod.Add(lr.Spec.Limits[0].Default[apiv1.ResourceCPU])
			} else {
				cpuPod.Add(c.Resources.Limits[apiv1.ResourceCPU])
			}
		} else {
			cpuPod.Add(v)
		}
	}
	if isDev {
		return cpuPod
	}
	cpuTotal := cpuPod
	for i := 1; int32(i) < replicas; i++ {
		cpuTotal.Add(cpuPod)
	}
	return cpuTotal
}

func matchMemory(dev *model.Dev, isDev bool, replicas int32, spec apiv1.PodSpec, lr *apiv1.LimitRange) resource.Quantity {
	memoryPod := resource.MustParse("0")
	for _, c := range spec.Containers {
		v, ok := dev.Resources.Limits[apiv1.ResourceMemory]
		if !isDev || !ok {
			if c.Resources.Limits == nil {
				memoryPod.Add(lr.Spec.Limits[0].Default[apiv1.ResourceMemory])
			} else {
				memoryPod.Add(c.Resources.Limits[apiv1.ResourceMemory])
			}
		} else {
			memoryPod.Add(v)
		}
	}
	if isDev {
		return memoryPod
	}
	meemoryTotal := memoryPod
	for i := 1; int32(i) < replicas; i++ {
		meemoryTotal.Add(memoryPod)
	}
	return meemoryTotal
}
