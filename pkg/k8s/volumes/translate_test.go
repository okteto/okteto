// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package volumes

import (
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_translate(t *testing.T) {
	filesystemVolumeMode := apiv1.PersistentVolumeFilesystem
	blockVolumeMode := apiv1.PersistentVolumeBlock
	storageClass := "okteto-sc"
	var tests = []struct {
		name string
		dev  *model.Dev
		want *apiv1.PersistentVolumeClaim
	}{
		{
			name: "default",
			dev: &model.Dev{
				Name: "test",
			},
			want: &apiv1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-okteto",
					Labels: map[string]string{
						constants.DevLabel: "true",
					},
				},
				Spec: apiv1.PersistentVolumeClaimSpec{
					AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
					VolumeMode:  &filesystemVolumeMode,
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"storage": resource.MustParse("5Gi"),
						},
					},
				},
			},
		},
		{
			name: "custom",
			dev: &model.Dev{
				Name: "test",
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled:      true,
					Labels:       map[string]string{"l1": "v1"},
					Annotations:  map[string]string{"a1": "v1"},
					AccessMode:   apiv1.ReadWriteMany,
					VolumeMode:   apiv1.PersistentVolumeBlock,
					Size:         "20Gi",
					StorageClass: storageClass,
				},
			},
			want: &apiv1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-okteto",
					Annotations: map[string]string{
						"a1": "v1",
					},
					Labels: map[string]string{
						"l1":               "v1",
						constants.DevLabel: "true",
					},
				},
				Spec: apiv1.PersistentVolumeClaimSpec{
					AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteMany},
					VolumeMode:  &blockVolumeMode,
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							"storage": resource.MustParse("20Gi"),
						},
					},
					StorageClassName: &storageClass,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translate(tt.dev)
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("wrong PVC generation in test '%s': '%v' vs '%v'", tt.name, result, tt.want)
			}
		})
	}
}
