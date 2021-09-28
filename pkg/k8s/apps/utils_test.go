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

package apps

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_set_translation_as_annotation_and_back(t *testing.T) {
	ctx := context.Background()
	manifest := []byte(`name: web
container: dev
image: web:latest
command: ["./run_web.sh"]
workdir: /app
annotations:
  key1: value1
  key2: value2`)
	dev, err := model.Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	d := GetDeploymentSandbox(dev)
	app := NewDeploymentApp(d)
	translations, err := GetTranslations(ctx, dev, app, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	tr1 := translations[d.Name]
	if err := setTranslationAsAnnotation(tr1); err != nil {
		t.Fatal(err)
	}
	translationString := d.Spec.Template.Annotations[model.TranslationAnnotation]
	if translationString == "" {
		t.Fatal("Marshalled translation was not found in the deployment's annotations")
	}
	tr2, err := getTranslationFromAnnotation(d.Spec.Template.Annotations)
	if err != nil {
		t.Fatal(err)
	}
	if tr1.Name != tr2.Name {
		t.Fatal("Mismatching Name value between original and unmarshalled translation")
	}
	if tr1.Version != tr2.Version {
		t.Fatal("Mismatching Version value between original and unmarshalled translation")
	}
	if tr1.Interactive != tr2.Interactive {
		t.Fatal("Mismatching Interactive flag between original and unmarshalled translation")
	}
	if tr1.Replicas != tr2.Replicas {
		t.Fatal("Mismatching Replicas count between original and unmarshalled translation")
	}
}

func Test_getPreviousAppReplicas(t *testing.T) {
	var twoReplica int32 = 2
	var tests = []struct {
		name     string
		app      App
		expected int32
	}{
		{
			name: "ok",
			app: NewDeploymentApp(
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: nil,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &twoReplica,
					},
				},
			),
			expected: 2,
		},
		{
			name: "sleeping-state-ok",
			app: NewDeploymentApp(
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: model.Annotations{
							model.StateBeforeSleepingAnnontation: "{\"Replicas\":3}",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &twoReplica,
					},
				},
			),
			expected: 3,
		},
		{
			name: "sleeping-state-ko",
			app: NewDeploymentApp(
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: model.Annotations{
							model.StateBeforeSleepingAnnontation: "wrong",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &twoReplica,
					},
				},
			),
			expected: 1,
		},
		{
			name: "ok-sfs",
			app: NewStatefulSetApp(
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: nil,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: &twoReplica,
					},
				},
			),
			expected: 2,
		},
		{
			name: "sleeping-state-ok-sfs",
			app: NewStatefulSetApp(
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: model.Annotations{
							model.StateBeforeSleepingAnnontation: "{\"Replicas\":3}",
						},
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: &twoReplica,
					},
				},
			),
			expected: 3,
		},
		{
			name: "sleeping-state-ko-sfs",
			app: NewStatefulSetApp(
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: model.Annotations{
							model.StateBeforeSleepingAnnontation: "wrong",
						},
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: &twoReplica,
					},
				},
			),
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPreviousAppReplicas(tt.app)
			if result != tt.expected {
				t.Errorf("Test '%s' failed: expected %d but got %d", tt.name, tt.expected, result)
			}
		})
	}
}
