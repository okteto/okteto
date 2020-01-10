package volumes

import (
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func translate(dev *model.Dev) *apiv1.PersistentVolumeClaim {
	pvc := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: dev.GetVolumeName(),
		},
		Spec: apiv1.PersistentVolumeClaimSpec{
			AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
			Resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					"storage": resource.MustParse(dev.PersistentVolumeSize()),
				},
			},
		},
	}
	if dev.PersistentVolumeStorageClass() != "" {
		storageClass := dev.PersistentVolumeStorageClass()
		pvc.Spec.StorageClassName = &storageClass
	}
	return pvc
}
