package preview

import (
	"context"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_ExecuteDeployPreview(t *testing.T) {

	tests := []struct {
		name              string
		username          string
		pipelineResponses *client.FakePipelineResponses
		previewResponses  *client.FakePreviewResponse
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
				ResourceStatus: map[string]string{},
			},
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
				},
			}
			err := pw.ExecuteDeployPreview(ctx, tt.opts)
			assert.ErrorIs(t, err, tt.expectedErr)

		})
	}

}
