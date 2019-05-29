package limitranges

import (
	"os"

	"github.com/okteto/app/api/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/resource"
)

func translate(s *model.Space) *apiv1.LimitRange {
	softMultitenancy := os.Getenv("OKTETO_SOFT_MULTITENANCY")
	if softMultitenancy == "YES" {
		return &apiv1.LimitRange{
			ObjectMeta: metav1.ObjectMeta{
				Name: s.ID,
			},
			Spec: apiv1.LimitRangeSpec{
				Limits: []apiv1.LimitRangeItem{
					apiv1.LimitRangeItem{
						Type: apiv1.LimitTypeContainer,
						Default: apiv1.ResourceList{
							apiv1.ResourceCPU:    resource.MustParse("2"),
							apiv1.ResourceMemory: resource.MustParse("4Gi"),
						},
						DefaultRequest: apiv1.ResourceList{
							apiv1.ResourceCPU:    resource.MustParse("0.250"),
							apiv1.ResourceMemory: resource.MustParse("0.5Gi"),
						},
					},
				},
			},
		}
	}
	return &apiv1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.ID,
		},
		Spec: apiv1.LimitRangeSpec{
			Limits: []apiv1.LimitRangeItem{
				apiv1.LimitRangeItem{
					Type: apiv1.LimitTypeContainer,
					Default: apiv1.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("1"),
						apiv1.ResourceMemory: resource.MustParse("2Gi"),
					},
					DefaultRequest: apiv1.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("0.125"),
						apiv1.ResourceMemory: resource.MustParse("0.250Gi"),
					},
				},
			},
		},
	}
}
