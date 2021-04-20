// Copyright 2020 The Okteto Authors
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

package pvcs

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/fake"
)

var ns = &apiv1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}

func TestCreate(t *testing.T) {
	ctx := context.Background()
	pvc := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	clientset := fake.NewSimpleClientset()
	err := Create(ctx, pvc, clientset)
	if err != nil {
		t.Fatal(err)
	}
	retrieved, err := clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, pvc.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retrieved, pvc) {
		t.Fatalf("Didn't created correctly")
	}
}

func TestList(t *testing.T) {
	ctx := context.Background()
	pvc := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	clientset := fake.NewSimpleClientset(pvc)
	pvcList, err := List(ctx, pvc.Namespace, "", clientset)
	if err != nil {
		t.Fatal(err)
	}

	if len(pvcList) != 1 {
		t.Fatal(fmt.Errorf("Expected 1 ingress, found %d", len(pvcList)))
	}

}

func TestDestroy(t *testing.T) {
	var tests = []struct {
		name      string
		pvcName   string
		namespace string
		pvc       *apiv1.PersistentVolumeClaim
	}{
		{
			name:      "existent-pvc",
			pvcName:   "pvc-test",
			namespace: "test",
			pvc: &apiv1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvc-test",
					Namespace: "test",
				},
			},
		},
		{
			name:      "pvc-not-found",
			pvcName:   "pvc-test",
			namespace: "test",
			pvc: &apiv1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-existent-pvc",
					Namespace: "another-space",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := fake.NewSimpleClientset(tt.pvc)

			err := Destroy(ctx, tt.pvcName, tt.namespace, client)

			if err != nil {
				t.Fatalf("unexpected error '%s'", err)
			}
		})
	}
}
