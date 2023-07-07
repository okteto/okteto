package preview

import (
	"context"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestExecuteDestroyPreviewWithErrorDestroying(t *testing.T) {
	ctx := context.Background()
	opts := &DestroyOptions{
		name: "test-preview",
		wait: true,
	}
	previewResponse := client.FakePreviewResponse{
		ErrDestroyPreview: assert.AnError,
	}
	command := destroyPreviewCommand{
		okClient: &client.FakeOktetoClient{
			Preview: client.NewFakePreviewClient(
				&previewResponse,
			),
			StreamClient: client.NewFakeStreamClient(&client.FakeStreamResponse{}),
		},
		k8sClient: fake.NewSimpleClientset(),
	}

	err := command.executeDestroyPreview(ctx, opts)

	require.Error(t, err)
	require.Equal(t, 0, previewResponse.DestroySuccessCount)
}

func TestExecuteDestroyPreviewWithoutError(t *testing.T) {
	ctx := context.Background()
	opts := &DestroyOptions{
		name: "test-preview",
		wait: true,
	}
	var previewResponse client.FakePreviewResponse
	command := destroyPreviewCommand{
		okClient: &client.FakeOktetoClient{
			Preview: client.NewFakePreviewClient(
				&previewResponse,
			),
			StreamClient: client.NewFakeStreamClient(&client.FakeStreamResponse{}),
		},
		k8sClient: fake.NewSimpleClientset(),
	}

	err := command.executeDestroyPreview(ctx, opts)

	require.NoError(t, err)
	require.Equal(t, 1, previewResponse.DestroySuccessCount)
}

func TestExecuteDestroyPreviewWithoutWait(t *testing.T) {
	ctx := context.Background()
	opts := &DestroyOptions{
		name: "test-preview",
		wait: false,
	}
	var previewResponse client.FakePreviewResponse
	command := destroyPreviewCommand{
		okClient: &client.FakeOktetoClient{
			Preview: client.NewFakePreviewClient(
				&previewResponse,
			),
			StreamClient: client.NewFakeStreamClient(&client.FakeStreamResponse{}),
		},
		k8sClient: fake.NewSimpleClientset(),
	}

	err := command.executeDestroyPreview(ctx, opts)

	require.NoError(t, err)
	require.Equal(t, 1, previewResponse.DestroySuccessCount)
}

func TestExecuteDestroyPreviewWithFailedJob(t *testing.T) {
	ctx := context.Background()
	opts := &DestroyOptions{
		name: "test-preview",
		wait: true,
	}
	var previewResponse client.FakePreviewResponse
	ns := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-preview",
			Labels: map[string]string{
				constants.NamespaceStatusLabel: "DeleteFailed",
			},
		},
	}
	command := destroyPreviewCommand{
		okClient: &client.FakeOktetoClient{
			Preview: client.NewFakePreviewClient(
				&previewResponse,
			),
			StreamClient: client.NewFakeStreamClient(&client.FakeStreamResponse{}),
		},
		k8sClient: fake.NewSimpleClientset(&ns),
	}

	err := command.executeDestroyPreview(ctx, opts)

	require.EqualError(t, err, errFailedDestroyPreview.Error())
	require.Equal(t, 1, previewResponse.DestroySuccessCount)
}

func TestExecuteDestroyPreviewWithErrorStreaming(t *testing.T) {
	ctx := context.Background()
	var previewResponse client.FakePreviewResponse
	opts := &DestroyOptions{
		name: "test-preview",
		wait: true,
	}
	command := destroyPreviewCommand{
		okClient: &client.FakeOktetoClient{
			Preview: client.NewFakePreviewClient(
				&previewResponse,
			),
			StreamClient: client.NewFakeStreamClient(&client.FakeStreamResponse{StreamErr: assert.AnError}),
		},
		k8sClient: fake.NewSimpleClientset(),
	}

	err := command.executeDestroyPreview(ctx, opts)

	require.NoError(t, err)
	require.Equal(t, 1, previewResponse.DestroySuccessCount)
}
