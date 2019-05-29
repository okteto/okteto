package redis

import (
	"github.com/okteto/app/api/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var devReplicas int32 = 1

//TranslateStatefulSet returns the statefulset for redis
func TranslateStatefulSet(db *model.DB, s *model.Space) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.REDIS,
			Namespace: s.ID,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: model.REDIS,
			Replicas:    &devReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": model.REDIS,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": model.REDIS,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						apiv1.Container{
							Name:            model.REDIS,
							Image:           model.REDIS,
							ImagePullPolicy: apiv1.PullIfNotPresent,
							Ports: []apiv1.ContainerPort{
								apiv1.ContainerPort{
									ContainerPort: 6379,
								},
							},
							VolumeMounts: []apiv1.VolumeMount{
								apiv1.VolumeMount{
									Name:      db.GetVolumeName(),
									MountPath: db.GetVolumePath(),
								},
							},
						},
					},
					Volumes: []apiv1.Volume{
						apiv1.Volume{
							Name: db.GetVolumeName(),
							VolumeSource: apiv1.VolumeSource{
								PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
									ClaimName: db.GetVolumeName(),
									ReadOnly:  false,
								},
							},
						},
					},
				},
			},
		},
	}
}
