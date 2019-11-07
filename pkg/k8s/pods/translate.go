package pods

import (
	"fmt"

	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	oktetoVolumeName = "okteto"
)

func translate(dev *model.Dev) *apiv1.Pod {
	return &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("volume-cleaner-%s", dev.Name),
			Namespace: dev.Namespace,
			Labels: map[string]string{
				okLabels.DevLabel: "true",
			},
		},
		Spec: apiv1.PodSpec{
			TerminationGracePeriodSeconds: &devTerminationGracePeriodSeconds,
			RestartPolicy:                 apiv1.RestartPolicyNever,
			Affinity: &apiv1.Affinity{
				PodAffinity: &apiv1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []apiv1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									okLabels.DevLabel: "true",
								},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
			Containers: []apiv1.Container{
				{
					Name:            "volume-cleaner",
					Image:           "busybox",
					ImagePullPolicy: apiv1.PullIfNotPresent,
					Command:         []string{"rm"},
					Args:            []string{"-Rf", fmt.Sprintf("/okteto/%s", dev.Name)},
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      oktetoVolumeName,
							MountPath: "/okteto",
						},
					},
					Resources: apiv1.ResourceRequirements{
						Limits: apiv1.ResourceList{
							apiv1.ResourceMemory: resource.MustParse("50Mi"),
							apiv1.ResourceCPU:    resource.MustParse("100m"),
						},
					},
				},
			},
			Volumes: []apiv1.Volume{
				{
					Name: oktetoVolumeName,
					VolumeSource: apiv1.VolumeSource{
						PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
							ClaimName: oktetoVolumeName,
							ReadOnly:  false,
						},
					},
				},
			},
		},
	}
}
