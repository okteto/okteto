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

package insights

import (
	"context"
	"fmt"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestNewInsightsPublisher(t *testing.T) {
	k8sClientProvider := test.NewFakeK8sProvider()
	ioCtrl := io.Controller{}

	publisher := NewInsightsPublisher(k8sClientProvider, ioCtrl)

	if publisher.k8sClientProvider != k8sClientProvider {
		t.Errorf("Expected k8sClientProvider to be %v, but got %v", k8sClientProvider, publisher.k8sClientProvider)
	}

	if publisher.ioCtrl != ioCtrl {
		t.Errorf("Expected ioCtrl to be %v, but got %v", ioCtrl, publisher.ioCtrl)
	}
}

func TestInsightsPublisher_trackEvent(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {
				Cfg: &api.Config{},
			},
		},
	}
	ip := &Publisher{
		k8sClientProvider: test.NewFakeK8sProvider(),
		ioCtrl:            io.Controller{},
	}

	ctx := context.TODO()
	namespace := "test-namespace"
	insightType := "test-insight"
	data := "test-data"

	c, _, err := ip.k8sClientProvider.Provide(&api.Config{})
	require.NoError(t, err)

	events, err := c.EventsV1().Events(namespace).List(ctx, v1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, events.Items, 0)
	ip.trackEvent(ctx, namespace, insightType, data)

	events, err = c.EventsV1().Events(namespace).List(ctx, v1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, events.Items, 1)

	e := events.Items[0]
	require.Equal(t, fmt.Sprintf("okteto-%s-", insightType), e.ObjectMeta.GenerateName)
	require.Equal(t, namespace, e.ObjectMeta.Namespace)
	require.Equal(t, "true", e.ObjectMeta.Labels["events.okteto.com"])
	require.Equal(t, fmt.Sprintf("okteto_insights_%s", insightType), e.Reason)
	require.Equal(t, reportingController, e.ReportingController)

}
