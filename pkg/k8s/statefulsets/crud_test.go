// Copyright 2021 The Okteto Authors
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

package statefulsets

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreate(t *testing.T) {
	ctx := context.Background()
	sfs := &appsv1.StatefulSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}

	clientset := fake.NewSimpleClientset()

	_, err := Create(ctx, sfs, clientset)
	if err != nil {
		t.Fatal(err)
	}
	retrieved, err := clientset.AppsV1().StatefulSets(sfs.Namespace).Get(ctx, sfs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retrieved, sfs) {
		t.Fatalf("Didn't created correctly")
	}
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	labels := map[string]string{"key": "value"}
	sfs := &appsv1.StatefulSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			Labels:    labels,
		},
	}

	clientset := fake.NewSimpleClientset(sfs)

	updatedLabels := map[string]string{"key": "value", "key2": "value2"}
	updatedsfs := &appsv1.StatefulSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			Labels:    updatedLabels,
		},
	}
	_, err := Update(ctx, updatedsfs, clientset)
	if err != nil {
		t.Fatal(err)
	}

	retrieved, err := clientset.AppsV1().StatefulSets(sfs.Namespace).Get(ctx, sfs.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retrieved, updatedsfs) {
		t.Fatalf("Didn't updated correctly")
	}
}

func TestDestroy(t *testing.T) {
	var tests = []struct {
		name      string
		sfsName   string
		namespace string
		sfs       *appsv1.StatefulSet
		deleted   bool
	}{
		{
			name:      "existent-sfs",
			sfsName:   "sfs-test",
			namespace: "test",
			sfs: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sfs-test",
					Namespace: "test",
				},
			},
			deleted: true,
		},
		{
			name:      "sfs-not-found",
			sfsName:   "sfs-test",
			namespace: "test",
			sfs: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-existent-sfs",
					Namespace: "another-space",
				},
			},
			deleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			clientset := fake.NewSimpleClientset(tt.sfs)

			err := Destroy(ctx, tt.sfsName, tt.namespace, clientset)

			if err != nil {
				t.Fatalf("unexpected error '%s'", err)
			}

			sfsList, err := List(ctx, tt.sfs.Namespace, "", clientset)
			if err != nil {
				t.Fatal(err)
			}
			if tt.deleted && len(sfsList) != 0 {
				t.Fatal("Not deleted")
			}
		})
	}
}

func TestList(t *testing.T) {
	ctx := context.Background()

	sfs := &appsv1.StatefulSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}
	clientset := fake.NewSimpleClientset(sfs)

	sfsList, err := List(ctx, sfs.Namespace, "", clientset)
	if err != nil {
		t.Fatal(err)
	}

	if len(sfsList) != 1 {
		t.Fatal(fmt.Errorf("Expected 1 sfs, found %d", len(sfsList)))
	}

}
