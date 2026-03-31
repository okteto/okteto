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

package apps

import (
	"context"
	"testing"

	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutsfake "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned/fake"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRolloutGetDevCloneWithError(t *testing.T) {
	r := &rolloutsv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}

	app := NewRolloutApp(r, rolloutsfake.NewSimpleClientset())
	c := fake.NewSimpleClientset()
	ctx := context.Background()

	_, err := app.GetDevClone(ctx, c)

	require.Error(t, err)
}

func TestRolloutGetDevCloneWithoutError(t *testing.T) {
	r := &rolloutsv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}

	cloned := &rolloutsv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.DevCloneName("test"),
			Namespace: "test",
			Labels: map[string]string{
				model.DevCloneLabel: "true",
			},
		},
	}

	rc := rolloutsfake.NewSimpleClientset(cloned)
	app := NewRolloutApp(r, rc)
	c := fake.NewSimpleClientset()
	ctx := context.Background()

	result, err := app.GetDevClone(ctx, c)

	require.NoError(t, err)
	require.Equal(t, model.DevCloneName("test"), result.ObjectMeta().Name)
	require.Equal(t, "test", result.ObjectMeta().Namespace)
}
