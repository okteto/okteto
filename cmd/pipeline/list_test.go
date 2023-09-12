package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func mockPipeline(fakeName string, fakeLabels []string) *apiv1.ConfigMap {
	var labels = map[string]string{
		model.GitDeployLabel: "true",
	}
	for _, label := range fakeLabels {
		key := fmt.Sprintf("%s/%s", constants.EnvironmentLabelKeyPrefix, label)
		labels[key] = "true"
	}

	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fakeName,
			Namespace: "test-ns",
			Labels:    labels,
		},
		Data: map[string]string{
			"name":       fakeName,
			"status":     fmt.Sprintf("%s-status", fakeName),
			"repository": fmt.Sprintf("%s-repository", fakeName),
			"branch":     fmt.Sprintf("%s-branch", fakeName),
		},
	}
}

func TestExecuteListPipelines(t *testing.T) {
	type input struct {
		flags                 listFlags
		listPipelines         listPipelinesFn
		getPipelineListOutput getPipelineListOutputFn
		c                     kubernetes.Interface
	}

	listPipelinesWithError := func(ctx context.Context, namespace, labelSelector string, c kubernetes.Interface) ([]apiv1.ConfigMap, error) {
		return nil, assert.AnError
	}

	tests := []struct {
		name                  string
		input                 input
		output                string
		expectedPrintedOutput string
		expectedError         error
	}{
		{
			name: "error - empty label",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "",
					labels:    []string{""},
				},
				listPipelines:         configmaps.List,
				getPipelineListOutput: getPipelineListOutput,
			},
			expectedError:         fmt.Errorf("invalid label: the provided label is empty"),
			expectedPrintedOutput: "",
		},
		{
			name: "error - invalid label",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "",
					labels:    []string{"@@@@@"},
				},
				listPipelines:         configmaps.List,
				getPipelineListOutput: getPipelineListOutput,
			},
			expectedError:         fmt.Errorf("invalid label '@@@@@': a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')"),
			expectedPrintedOutput: "",
		},
		{
			name: "error - retrieving pipelines",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "",
				},
				listPipelines:         listPipelinesWithError,
				getPipelineListOutput: getPipelineListOutput,
				c: fake.NewSimpleClientset(
					&apiv1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ns",
							Labels: map[string]string{
								constants.NamespaceStatusLabel: "Deployed",
							},
						},
					},
				),
			},
			expectedError:         assert.AnError,
			expectedPrintedOutput: "",
		},
		{
			name: "success - text output - without label filter",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "",
				},
				listPipelines:         configmaps.List,
				getPipelineListOutput: getPipelineListOutput,
				c: fake.NewSimpleClientset(
					&apiv1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ns",
							Labels: map[string]string{
								constants.NamespaceStatusLabel: "Deployed",
							},
						},
					},
					mockPipeline("dev1", []string{}),
					mockPipeline("dev2", []string{"fake-label-2"}),
					mockPipeline("dev3", []string{"fake-label-3"}),
				),
			},
			expectedError: nil,
			expectedPrintedOutput: `Name  Status       Repository       Branch       Labels
dev1  dev1-status  dev1-repository  dev1-branch  -
dev2  dev2-status  dev2-repository  dev2-branch  fake-label-2
dev3  dev3-status  dev3-repository  dev3-branch  fake-label-3
`,
		},
		{
			name: "success - text output - with label filter",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "",
					labels:    []string{"fake-label-2"},
				},
				listPipelines:         configmaps.List,
				getPipelineListOutput: getPipelineListOutput,
				c: fake.NewSimpleClientset(
					&apiv1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ns",
							Labels: map[string]string{
								constants.NamespaceStatusLabel: "Deployed",
							},
						},
					},
					mockPipeline("dev1", []string{}),
					mockPipeline("dev2", []string{"fake-label-2"}),
					mockPipeline("dev3", []string{"fake-label-3"}),
				),
			},
			expectedError: nil,
			expectedPrintedOutput: `Name  Status       Repository       Branch       Labels
dev2  dev2-status  dev2-repository  dev2-branch  fake-label-2
`,
		},
		{
			name: "success - JSON output",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "json",
				},
				listPipelines:         configmaps.List,
				getPipelineListOutput: getPipelineListOutput,
				c: fake.NewSimpleClientset(
					&apiv1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ns",
							Labels: map[string]string{
								constants.NamespaceStatusLabel: "Deployed",
							},
						},
					},
					mockPipeline("dev1", []string{}),
					mockPipeline("dev2", []string{"fake-label-2"}),
					mockPipeline("dev3", []string{"fake-label-3"}),
				),
			},
			expectedError: nil,
			expectedPrintedOutput: `[
 {
  "name": "dev1",
  "status": "dev1-status",
  "repository": "dev1-repository",
  "branch": "dev1-branch",
  "labels": []
 },
 {
  "name": "dev2",
  "status": "dev2-status",
  "repository": "dev2-repository",
  "branch": "dev2-branch",
  "labels": [
   "fake-label-2"
  ]
 },
 {
  "name": "dev3",
  "status": "dev3-status",
  "repository": "dev3-repository",
  "branch": "dev3-branch",
  "labels": [
   "fake-label-3"
  ]
 }
]`,
		},
		{
			name: "success - empty JSON output",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "json",
				},
				listPipelines:         configmaps.List,
				getPipelineListOutput: getPipelineListOutput,
				c: fake.NewSimpleClientset(
					&apiv1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ns",
							Labels: map[string]string{
								constants.NamespaceStatusLabel: "Deployed",
							},
						},
					},
				),
			},
			expectedError:         nil,
			expectedPrintedOutput: `[]`,
		},
		{
			name: "success - YAML output",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "yaml",
				},
				listPipelines:         configmaps.List,
				getPipelineListOutput: getPipelineListOutput,
				c: fake.NewSimpleClientset(
					&apiv1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ns",
							Labels: map[string]string{
								constants.NamespaceStatusLabel: "Deployed",
							},
						},
					},
					mockPipeline("dev1", []string{}),
					mockPipeline("dev2", []string{"fake-label-2"}),
					mockPipeline("dev3", []string{"fake-label-3"}),
				),
			},
			expectedError: nil,
			expectedPrintedOutput: `- name: dev1
  status: dev1-status
  repository: dev1-repository
  branch: dev1-branch
  labels: []
- name: dev2
  status: dev2-status
  repository: dev2-repository
  branch: dev2-branch
  labels:
  - fake-label-2
- name: dev3
  status: dev3-status
  repository: dev3-repository
  branch: dev3-branch
  labels:
  - fake-label-3
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Redirect the log output to a buffer for our test
			var buf bytes.Buffer

			err := executeListPipelines(context.Background(), tt.input.flags, tt.input.listPipelines, tt.input.getPipelineListOutput, tt.input.c, &buf)

			if tt.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			}

			assert.Equal(t, tt.expectedPrintedOutput, buf.String())
		})
	}
}

func TestGetPipelineListOutput(t *testing.T) {
	type input struct {
		flags         listFlags
		namespace     string
		labels        []string
		listPipelines listPipelinesFn
		c             kubernetes.Interface
	}

	type output struct {
		pipelines []pipelineListItem
		err       error
	}

	listPipelinesWithError := func(ctx context.Context, namespace, labelSelector string, c kubernetes.Interface) ([]apiv1.ConfigMap, error) {
		return nil, assert.AnError
	}

	tests := []struct {
		name   string
		input  input
		output output
	}{
		{
			name: "success - empty list",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "",
				},
				labels:        []string{},
				namespace:     "test-ns",
				listPipelines: configmaps.List,
				c: fake.NewSimpleClientset(&apiv1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
						Labels: map[string]string{
							constants.NamespaceStatusLabel: "Deployed",
						},
					},
				}),
			},
			output: output{
				err: nil,
			},
		},
		{
			name: "success - 3 dev envs",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "",
				},
				labels:        []string{},
				namespace:     "test-ns",
				listPipelines: configmaps.List,
				c: fake.NewSimpleClientset(
					&apiv1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ns",
							Labels: map[string]string{
								constants.NamespaceStatusLabel: "Deployed",
							},
						},
					},
					mockPipeline("dev1", []string{}),
					mockPipeline("dev2", []string{"fake-label-2"}),
					mockPipeline("dev3", []string{"fake-label-3"}),
				),
			},
			output: output{
				pipelines: []pipelineListItem{
					{
						Name:       "dev1",
						Status:     "dev1-status",
						Repository: "dev1-repository",
						Branch:     "dev1-branch",
						Labels:     []string{},
					},
					{
						Name:       "dev2",
						Status:     "dev2-status",
						Repository: "dev2-repository",
						Branch:     "dev2-branch",
						Labels: []string{
							"fake-label-2",
						},
					},
					{
						Name:       "dev3",
						Status:     "dev3-status",
						Repository: "dev3-repository",
						Branch:     "dev3-branch",
						Labels: []string{
							"fake-label-3",
						},
					},
				},
				err: nil,
			},
		},
		{
			name: "success - 3 dev envs - sleeping",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "",
				},
				labels:        []string{},
				namespace:     "test-ns",
				listPipelines: configmaps.List,
				c: fake.NewSimpleClientset(
					&apiv1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ns",
							Labels: map[string]string{
								constants.NamespaceStatusLabel: "Sleeping",
							},
						},
					},
					mockPipeline("dev1", []string{}),
					mockPipeline("dev2", []string{"fake-label-2"}),
					mockPipeline("dev3", []string{"fake-label-3"}),
				),
			},
			output: output{
				pipelines: []pipelineListItem{
					{
						Name:       "dev1",
						Status:     "Sleeping",
						Repository: "dev1-repository",
						Branch:     "dev1-branch",
						Labels:     []string{},
					},
					{
						Name:       "dev2",
						Status:     "Sleeping",
						Repository: "dev2-repository",
						Branch:     "dev2-branch",
						Labels: []string{
							"fake-label-2",
						},
					},
					{
						Name:       "dev3",
						Status:     "Sleeping",
						Repository: "dev3-repository",
						Branch:     "dev3-branch",
						Labels: []string{
							"fake-label-3",
						},
					},
				},
				err: nil,
			},
		},
		{
			name: "error - cannot get namespace",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "",
				},
				labels:        []string{},
				namespace:     "test-ns",
				listPipelines: configmaps.List,
				c:             fake.NewSimpleClientset(),
			},
			output: output{
				err: fmt.Errorf("namespaces \"test-ns\" not found"),
			},
		},
		{
			name: "error - cannot get pipelines",
			input: input{
				flags: listFlags{
					namespace: "test-ns",
					output:    "",
				},
				labels:        []string{},
				namespace:     "test-ns",
				listPipelines: listPipelinesWithError,
				c: fake.NewSimpleClientset(&apiv1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
						Labels: map[string]string{
							constants.NamespaceStatusLabel: "Deployed",
						},
					},
				}),
			},
			output: output{
				err: assert.AnError,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			labelSelector, err := getLabelSelector(tt.input.labels)
			assert.NoError(t, err)

			pipelines, err := getPipelineListOutput(ctx, tt.input.listPipelines, tt.input.namespace, labelSelector, tt.input.c)

			assert.Equal(t, tt.output.pipelines, pipelines)

			if tt.output.err == nil {
				assert.NoError(t, err)
			} else {
				assert.Equal(t, tt.output.err.Error(), err.Error())
			}
		})
	}
}
