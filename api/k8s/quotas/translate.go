package quotas

import (
	"os"

	"github.com/okteto/app/api/model"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func translate(s *model.Space) *apiv1.ResourceQuota {
	softMultitenancy := os.Getenv("OKTETO_SOFT_MULTITENANCY")
	if softMultitenancy == "YES" {
		return &apiv1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name: s.ID,
			},
			Spec: apiv1.ResourceQuotaSpec{
				Hard: map[apiv1.ResourceName]resource.Quantity{
					apiv1.ResourceCPU:                      resource.MustParse("8"),
					apiv1.ResourceMemory:                   resource.MustParse("10Gi"),
					apiv1.ResourcePods:                     resource.MustParse("50"),
					apiv1.ResourceServices:                 resource.MustParse("50"),
					apiv1.ResourceReplicationControllers:   resource.MustParse("50"),
					apiv1.ResourceQuotas:                   resource.MustParse("3"),
					apiv1.ResourceSecrets:                  resource.MustParse("50"),
					apiv1.ResourceConfigMaps:               resource.MustParse("50"),
					apiv1.ResourcePersistentVolumeClaims:   resource.MustParse("15"),
					apiv1.ResourceServicesNodePorts:        resource.MustParse("10"),
					apiv1.ResourceServicesLoadBalancers:    resource.MustParse("10"),
					apiv1.ResourceRequestsCPU:              resource.MustParse("8"),
					apiv1.ResourceRequestsMemory:           resource.MustParse("10Gi"),
					apiv1.ResourceRequestsStorage:          resource.MustParse("250Gi"),
					apiv1.ResourceRequestsEphemeralStorage: resource.MustParse("30Gi"),
					apiv1.ResourceLimitsCPU:                resource.MustParse("8"),
					apiv1.ResourceLimitsMemory:             resource.MustParse("10Gi"),
					apiv1.ResourceLimitsEphemeralStorage:   resource.MustParse("30Gi"),
				},
			},
		}
	}
	return &apiv1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.ID,
		},
		Spec: apiv1.ResourceQuotaSpec{
			Hard: map[apiv1.ResourceName]resource.Quantity{
				apiv1.ResourceCPU:                      resource.MustParse("3"),
				apiv1.ResourceMemory:                   resource.MustParse("5Gi"),
				apiv1.ResourcePods:                     resource.MustParse("10"),
				apiv1.ResourceServices:                 resource.MustParse("10"),
				apiv1.ResourceReplicationControllers:   resource.MustParse("5"),
				apiv1.ResourceQuotas:                   resource.MustParse("3"),
				apiv1.ResourceSecrets:                  resource.MustParse("10"),
				apiv1.ResourceConfigMaps:               resource.MustParse("10"),
				apiv1.ResourcePersistentVolumeClaims:   resource.MustParse("15"),
				apiv1.ResourceServicesNodePorts:        resource.MustParse("0"),
				apiv1.ResourceServicesLoadBalancers:    resource.MustParse("0"),
				apiv1.ResourceRequestsCPU:              resource.MustParse("3"),
				apiv1.ResourceRequestsMemory:           resource.MustParse("5Gi"),
				apiv1.ResourceRequestsStorage:          resource.MustParse("150Gi"),
				apiv1.ResourceRequestsEphemeralStorage: resource.MustParse("30Gi"),
				apiv1.ResourceLimitsCPU:                resource.MustParse("3"),
				apiv1.ResourceLimitsMemory:             resource.MustParse("5Gi"),
				apiv1.ResourceLimitsEphemeralStorage:   resource.MustParse("30Gi"),
			},
		},
	}
}
