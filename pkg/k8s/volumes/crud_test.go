package volumes

import (
	"testing"

	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_checkPVCValues(t *testing.T) {
	className := "class"
	var tests = []struct {
		name      string
		pvc       *apiv1.PersistentVolumeClaim
		dev       *model.Dev
		wantError bool
	}{
		{
			name: "ok-without-storage-class",
			pvc: &apiv1.PersistentVolumeClaim{
				Spec: apiv1.PersistentVolumeClaimSpec{
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"storage": resource.MustParse("20Gi"),
						},
					},
				},
			},
			dev: &model.Dev{
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Size: "20Gi",
				},
			},
			wantError: false,
		},
		{
			name: "ok-with-storage-class",
			pvc: &apiv1.PersistentVolumeClaim{
				Spec: apiv1.PersistentVolumeClaimSpec{
					StorageClassName: &className,
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"storage": resource.MustParse("20Gi"),
						},
					},
				},
			},
			dev: &model.Dev{
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Size:         "20Gi",
					StorageClass: "class",
				},
			},
			wantError: false,
		},
		{
			name: "pvc-without-storage",
			pvc: &apiv1.PersistentVolumeClaim{
				Spec: apiv1.PersistentVolumeClaimSpec{
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"cpu": resource.MustParse("1"),
						},
					},
				},
			},
			dev: &model.Dev{
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Size:         "20Gi",
					StorageClass: "class",
				},
			},
			wantError: true,
		},
		{
			name: "pvc-with-wrong-storage-size",
			pvc: &apiv1.PersistentVolumeClaim{
				Spec: apiv1.PersistentVolumeClaimSpec{
					StorageClassName: &className,
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"storage": resource.MustParse("20Gi"),
						},
					},
				},
			},
			dev: &model.Dev{
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Size:         "30Gi",
					StorageClass: "class",
				},
			},
			wantError: true,
		},
		{
			name: "pvc-with-wrong-storage-class",
			pvc: &apiv1.PersistentVolumeClaim{
				Spec: apiv1.PersistentVolumeClaimSpec{
					StorageClassName: &className,
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"storage": resource.MustParse("20Gi"),
						},
					},
				},
			},
			dev: &model.Dev{
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Size:         "20Gi",
					StorageClass: "wrong-class",
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkPVCValues(tt.pvc, tt.dev)
			if err == nil && tt.wantError {
				t.Errorf("checkPVCValues in test '%s' did not fail", tt.name)
			}
			if err != nil && !tt.wantError {
				t.Errorf("checkPVCValues in test '%s' failed: %s", tt.name, err)
			}
		})
	}
}
