package volume

import (
	"cli/cnd/pkg/model"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func translate(devList []*model.Dev) []*apiv1.PersistentVolumeClaim {
	result := []*apiv1.PersistentVolumeClaim{}
	for _, dev := range devList {
		quantDisk, _ := resource.ParseQuantity(dev.WorkDir.Size)
		result = append(
			result,
			&apiv1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: dev.GetCNDSyncVolume(),
				},
				Spec: apiv1.PersistentVolumeClaimSpec{
					AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"storage": quantDisk,
						},
					},
				},
			},
		)
		for _, v := range dev.Volumes {
			quantDisk, _ := resource.ParseQuantity(v.Size)
			result = append(
				result,
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: dev.GetCNDDataVolume(v),
					},
					Spec: apiv1.PersistentVolumeClaimSpec{
						AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
						Resources: apiv1.ResourceRequirements{
							Requests: apiv1.ResourceList{
								"storage": quantDisk,
							},
						},
					},
				},
			)
		}
		if dev.EnableDocker {
			quantDisk, _ := resource.ParseQuantity("30Gi")
			result = append(
				result,
				&apiv1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: dev.GetCNDDinDVolume(),
					},
					Spec: apiv1.PersistentVolumeClaimSpec{
						AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
						Resources: apiv1.ResourceRequirements{
							Requests: apiv1.ResourceList{
								"storage": quantDisk,
							},
						},
					},
				},
			)
		}
	}
	return result
}
