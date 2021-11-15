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

package endpoints

import (
	"context"
	"fmt"
	"testing"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/fake"
)

func TestList(t *testing.T) {
	ctx := context.Background()
	e := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	clientset := fake.NewSimpleClientset(e)

	iList, err := List(ctx, e.Namespace, "", clientset)
	if err != nil {
		t.Fatal(err)
	}

	if len(iList) != 1 {
		t.Fatal(fmt.Errorf("Expected 1 ingress, found %d", len(iList)))
	}
}
