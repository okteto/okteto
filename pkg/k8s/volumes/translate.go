package volumes

import (
	"log"
	"os"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	oktetoVolumName = "okteto"
)

func translate() *apiv1.PersistentVolumeClaim {
	quantDisk := getVolumeSize()
	return &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: oktetoVolumName,
		},
		Spec: apiv1.PersistentVolumeClaimSpec{
			AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
			Resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					"storage": quantDisk,
				},
			},
		},
	}
}

func getVolumeSize() resource.Quantity {
	quantDisk, _ := resource.ParseQuantity("10Gi")
	if size, ok := os.LookupEnv("OKTETO_VOLUME_SIZE"); ok {
		q, err := resource.ParseQuantity(size)
		if err != nil {
			log.Fatalf("%s is not a valid quantity", err)
		}
		quantDisk = q
	}
	return quantDisk
}
