package mongo

import (
	"github.com/okteto/app/api/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var devReplicas int32 = 1

//TranslateStatefulSet returns the statefulset for mongo
func TranslateStatefulSet(db *model.DB, s *model.Space) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.MONGO,
			Namespace: s.ID,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: model.MONGO,
			Replicas:    &devReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": model.MONGO,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": model.MONGO,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						apiv1.Container{
							Name:            model.MONGO,
							Image:           model.MONGO,
							ImagePullPolicy: apiv1.PullIfNotPresent,
							Ports: []apiv1.ContainerPort{
								apiv1.ContainerPort{
									ContainerPort: 27017,
								},
							},
						},
					},
				},
			},
		},
	}
}
