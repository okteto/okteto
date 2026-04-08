// Copyright 2026 The Okteto Authors
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

package rollouts

import (
	"context"
	"fmt"
	"testing"

	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutsfake "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned/fake"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGetByDev(t *testing.T) {
	tests := []struct {
		name          string
		dev           *model.Dev
		rollouts      []*rolloutsv1alpha1.Rollout
		expectedName  string
		expectedError error
	}{
		{
			name: "get by name",
			dev:  &model.Dev{Name: "test"},
			rollouts: []*rolloutsv1alpha1.Rollout{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"},
				},
			},
			expectedName: "test",
		},
		{
			name: "get by selector ignores dev clone",
			dev: &model.Dev{
				Name:     "test",
				Selector: map[string]string{"app": "test"},
			},
			rollouts: []*rolloutsv1alpha1.Rollout{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Labels:    map[string]string{"app": "test"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto",
						Namespace: "test",
						Labels: map[string]string{
							"app":               "test",
							model.DevCloneLabel: "clone",
						},
					},
				},
			},
			expectedName: "test",
		},
		{
			name: "multiple rollouts",
			dev: &model.Dev{
				Name:     "test",
				Selector: map[string]string{"app": "test"},
			},
			rollouts: []*rolloutsv1alpha1.Rollout{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-1", Namespace: "test", Labels: map[string]string{"app": "test"}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-2", Namespace: "test", Labels: map[string]string{"app": "test"}},
				},
			},
			expectedError: fmt.Errorf("found '2' rollouts for labels 'app=test' instead of 1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []runtime.Object
			for _, r := range tt.rollouts {
				objects = append(objects, r)
			}

			client := rolloutsfake.NewSimpleClientset(objects...)
			result, err := GetByDev(context.Background(), tt.dev, "test", client)

			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedName, result.Name)
		})
	}
}

func TestDeployAndPatchAnnotations(t *testing.T) {
	ctx := context.Background()
	r := &rolloutsv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test",
			Namespace:   "test",
			Annotations: map[string]string{"before": "value"},
		},
	}

	client := rolloutsfake.NewSimpleClientset()

	created, err := Deploy(ctx, r, client)
	assert.NoError(t, err)
	assert.Equal(t, "test", created.Name)

	created.Annotations["after"] = "value"
	err = PatchAnnotations(ctx, created, client)
	assert.NoError(t, err)

	updated, err := Get(ctx, "test", "test", client)
	assert.NoError(t, err)
	assert.Equal(t, "value", updated.Annotations["after"])
}

func TestDeployUpdatesExistingRollout(t *testing.T) {
	ctx := context.Background()
	existing := &rolloutsv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "test",
			ResourceVersion: "7",
			Annotations:     map[string]string{"before": "value"},
		},
	}

	client := rolloutsfake.NewSimpleClientset(existing.DeepCopy())

	existing.Annotations["after"] = "value"
	updated, err := Deploy(ctx, existing, client)
	assert.NoError(t, err)
	assert.Equal(t, "value", updated.Annotations["after"])

	reloaded, err := Get(ctx, "test", "test", client)
	assert.NoError(t, err)
	assert.Equal(t, "value", reloaded.Annotations["after"])
}

func TestDeployHydratesResourceVersionBeforeUpdate(t *testing.T) {
	ctx := context.Background()
	existing := &rolloutsv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "test",
			ResourceVersion: "11",
			Annotations:     map[string]string{"before": "value"},
		},
	}

	client := rolloutsfake.NewSimpleClientset(existing.DeepCopy())

	desired := existing.DeepCopy()
	desired.ResourceVersion = ""
	desired.Annotations["after"] = "value"

	updated, err := Deploy(ctx, desired, client)
	assert.NoError(t, err)
	assert.Equal(t, "11", updated.ResourceVersion)

	reloaded, err := Get(ctx, "test", "test", client)
	assert.NoError(t, err)
	assert.Equal(t, "value", reloaded.Annotations["after"])
}

func TestCheckConditionErrors(t *testing.T) {
	r := &rolloutsv1alpha1.Rollout{
		Status: rolloutsv1alpha1.RolloutStatus{
			Conditions: []rolloutsv1alpha1.RolloutCondition{
				{
					Type:    rolloutsv1alpha1.RolloutReplicaFailure,
					Reason:  "FailedCreate",
					Status:  apiv1.ConditionTrue,
					Message: "exceeded quota: requested: pods=1",
				},
			},
		},
	}

	err := CheckConditionErrors(r, &model.Dev{})
	assert.EqualError(t, err, "quota exceeded, you have reached the maximum number of pods per namespace")
}
