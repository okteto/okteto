package preview

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_ExecuteDeployPreview(t *testing.T) {

	errWait := errors.New("wait-error")
	errResources := errors.New("resources-error")
	errDeployPreview := errors.New("fake deploy error")

	tests := []struct {
		name              string
		username          string
		pipelineResponses *client.FakePipelineResponses
		previewResponses  *client.FakePreviewResponse
		streamResponses      *client.FakeStreamResponse
		opts              *DeployOptions
		expectedErr       error
	}{
		{
			name:     "success",
			username: "test-username",
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test-repo",
				branch:     "test-branch",
			},
			pipelineResponses: &client.FakePipelineResponses{},
			previewResponses: &client.FakePreviewResponse{
				Preview: &types.PreviewResponse{
					Action: &types.Action{
						Name: "action-name",
					},
				},
			},
		},
		{
			name:     "success-wait",
			username: "test-username",
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test-repo",
				branch:     "test-branch",
				wait:       true,
				timeout:    1 * time.Minute,
			},
			pipelineResponses: &client.FakePipelineResponses{},
			previewResponses: &client.FakePreviewResponse{
				Preview: &types.PreviewResponse{
					Action: &types.Action{
						Name: "action-name",
					},
				},
				ResourceStatus: map[string]string{},
			},
			streamResponses: &client.FakeStreamResponse{},
		},
		{
			name:     "success-wait-stream-err",
			username: "test-username",
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test-repo",
				branch:     "test-branch",
				wait:       true,
				timeout:    1 * time.Minute,
			},
			pipelineResponses: &client.FakePipelineResponses{},
			streamResponses: &client.FakeStreamResponse{
				StreamErr: errors.New("error"),
			},
			previewResponses: &client.FakePreviewResponse{
				Preview: &types.PreviewResponse{
					Action: &types.Action{
						Name: "action-name",
					},
				},
				ResourceStatus: map[string]string{},
			},
		},
		{
			name:     "err-deploy-preview",
			username: "test-username",
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test-repo",
				branch:     "test-branch",
			},
			pipelineResponses: &client.FakePipelineResponses{},
			previewResponses: &client.FakePreviewResponse{
				ErrDeployPreview: errDeployPreview,
			},
			expectedErr: errDeployPreview,
		},
		{
			name:     "err-wait-deploy",
			username: "test-username",
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test-repo",
				branch:     "test-branch",
				wait:       true,
			},
			pipelineResponses: &client.FakePipelineResponses{
				WaitErr: errWait,
			},
			previewResponses: &client.FakePreviewResponse{
				Preview: &types.PreviewResponse{
					Action: &types.Action{
						Name: "action-name",
					},
				},
			},
			streamResponses: &client.FakeStreamResponse{},
			expectedErr:  errWait,
		},
		{
			name:     "err-wait-resources",
			username: "test-username",
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test-repo",
				branch:     "test-branch",
				wait:       true,
				timeout:    1 * time.Minute,
			},
			pipelineResponses: &client.FakePipelineResponses{},
			previewResponses: &client.FakePreviewResponse{
				Preview: &types.PreviewResponse{
					Action: &types.Action{
						Name: "action-name",
					},
				},
				ErrResources: errResources,
			},
			streamResponses: &client.FakeStreamResponse{},
			expectedErr:  errResources,
		},
		{
			name:     "err-wait-resources-timeout",
			username: "test-username",
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test-repo",
				branch:     "test-branch",
				wait:       true,
				timeout:    1 * time.Second,
			},
			pipelineResponses: &client.FakePipelineResponses{},
			previewResponses: &client.FakePreviewResponse{
				Preview: &types.PreviewResponse{
					Action: &types.Action{
						Name: "action-name",
					},
				},
			},
			streamResponses: &client.FakeStreamResponse{},
			expectedErr:  ErrWaitResourcesTimeout,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			okteto.CurrentStore = &okteto.OktetoContextStore{
				CurrentContext: "test",
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Username: tt.username,
					},
				},
			}
			pw := &Command{
				okClient: &client.FakeOktetoClient{
					PipelineClient: client.NewFakePipelineClient(tt.pipelineResponses),
					Preview:        client.NewFakePreviewClient(tt.previewResponses),
					StreamClient:   client.NewFakeStreamClient(tt.streamResponses),
				},
			}
			err := pw.ExecuteDeployPreview(ctx, tt.opts)
			assert.ErrorIs(t, err, tt.expectedErr)

		})
	}

}
