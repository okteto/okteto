package pipeline

import (
	"bytes"
	"context"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"testing"
)

func mockPipeline(name string) apiv1.ConfigMap {
	return apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				model.GitDeployLabel: "true",
			},
		},
	}
}

func TestExecuteListPipelines(t *testing.T) {
	type input struct {
		flags                 listFlags
		listPipelines         listPipelinesFn
		getPipelineListOutput getPipelineListOutputFn
		labelSelector         string
		c                     kubernetes.Interface
	}

	mockFuncPipelines3 := func(ctx context.Context, namespace, labelSelector string, c kubernetes.Interface) ([]apiv1.ConfigMap, error) {
		return []apiv1.ConfigMap{
			mockPipeline("test-pipeline-1"),
			mockPipeline("test-pipeline-2"),
			mockPipeline("test-pipeline-3"),
		}, nil
	}

	tests := []struct {
		name   string
		input  input
		output string
		//mockPipelineListOutput    []pipelineListItem
		//mockPipelineListOutputErr error
		//expectedPrintedOutput     string
		//expectedError             error
	}{
		{
			name: "text - success - not empty",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "",
				},
				listPipelines:         mockFuncPipelines3,
				getPipelineListOutput: getPipelineListOutput,
				labelSelector:         model.GitDeployLabel,
				c:                     nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockGetPipelineListOutput := func(ctx context.Context, listPipelines listPipelinesFn, namespace, labelSelector string, c kubernetes.Interface) ([]pipelineListItem, error) {
				return tt.mockPipelineListOutput, tt.mockPipelineListOutputErr
			}

			pc := &Command{}

			// Redirect the log output to a buffer for our test
			var buf bytes.Buffer
			oktetoLog.SetOutput(&buf)

			err := pc.executeListPipelines(context.Background(), listFlags{output: tt.output}, nil, mockGetPipelineListOutput, "", nil)

			if tt.expectedError != nil {
				if err == nil || err.Error() != tt.expectedError.Error() {
					t.Fatalf("expected error '%v', but got '%v'", tt.expectedError, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if buf.String() != tt.expectedPrintedOutput {
				t.Fatalf("unexpected output: got %v, want %v", buf.String(), tt.expectedPrintedOutput)
			}
		})
	}
}
