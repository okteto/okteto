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

package apps

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSfsGetDevCloneWithError(t *testing.T) {
	sfs := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}
	app := StatefulSetApp{kind: okteto.StatefulSet, sfs: sfs}
	c := fake.NewSimpleClientset()
	ctx := context.Background()

	_, err := app.GetDevClone(ctx, c)

	require.Error(t, err)
}

func TestSfsGetDevCloneWithoutError(t *testing.T) {
	sfs := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}

	cloned := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-okteto",
			Namespace: "test",
			Labels: map[string]string{
				"dev.okteto.com/clone": "true",
			},
		},
	}

	app := StatefulSetApp{kind: okteto.StatefulSet, sfs: sfs}
	c := fake.NewSimpleClientset(cloned)
	ctx := context.Background()
	expected := &StatefulSetApp{kind: okteto.StatefulSet, sfs: cloned}

	result, err := app.GetDevClone(ctx, c)

	require.NoError(t, err)
	require.Equal(t, expected, result)
}
