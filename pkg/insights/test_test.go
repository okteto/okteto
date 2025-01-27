package insights

import (
	"context"
	"testing"
	"time"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestConvertTestdMetadataToEvent(t *testing.T) {
	metadata := &analytics.SingleTestMetadata{
		DevenvName: "test-devenv",
		TestName:   "test-test",
		Repository: "test-repo",
		Namespace:  "test-namespace",
		Duration:   time.Duration(5) * time.Second,
		Success:    true,
	}

	publisher := &Publisher{}
	expectedEvent := testEventJSON{
		DevenvName:    "test-devenv",
		Namespace:     "test-namespace",
		Repository:    "test-repo",
		Success:       true,
		Duration:      5.0,
		SchemaVersion: "1.0",
		TestName:      "test-test",
	}

	event := publisher.convertTestMetadataToEvent(metadata)
	assert.Equal(t, expectedEvent, event)
}

func TestTrackTest(t *testing.T) {
	ctx := context.Background()

	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {
				Cfg: &api.Config{},
			},
		},
	}

	ip := &Publisher{
		ioCtrl:            *io.NewIOController(),
		k8sClientProvider: test.NewFakeK8sProvider(),
	}

	meta := &analytics.SingleTestMetadata{
		DevenvName: "test-devenv",
		TestName:   "test-test",
		Repository: "test-repo",
		Namespace:  "test-namespace",
		Duration:   time.Duration(5) * time.Second,
		Success:    true,
	}

	c, _, err := ip.k8sClientProvider.Provide(&api.Config{})
	require.NoError(t, err)

	events, err := c.EventsV1().Events("test-namespace").List(ctx, v1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, events.Items, 0)

	ip.TrackTest(ctx, meta)

	events, err = c.EventsV1().Events("test-namespace").List(ctx, v1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, events.Items, 1)
	insightEvent := events.Items[0]
	assert.Equal(t, "okteto_insights_build", insightEvent.Reason)
	assert.Equal(t, "Normal", insightEvent.Type)
	assert.Equal(t, "build", insightEvent.Action)
	assert.Contains(t, "build", insightEvent.Name)
}
