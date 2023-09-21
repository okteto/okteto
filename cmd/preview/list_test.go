package preview

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_getPreviewOutput(t *testing.T) {
	var tests = []struct {
		name           string
		previews       []types.Preview
		expectedResult []previewOutput
	}{
		{
			name: "List all previews",
			previews: []types.Preview{
				{
					ID:            "test",
					PreviewLabels: []string{"label-1", "label-2"},
				},
				{
					ID:            "test-1",
					PreviewLabels: []string{"label-3"},
				},
				{
					ID:            "test-2",
					Sleeping:      true,
					PreviewLabels: []string{"-"},
				},
			},
			expectedResult: []previewOutput{
				{
					Name:     "test",
					Scope:    "",
					Sleeping: false,
					Labels:   []string{"label-1", "label-2"},
				},
				{
					Name:     "test-1",
					Scope:    "",
					Sleeping: false,
					Labels:   []string{"label-3"},
				},
				{
					Name:     "test-2",
					Scope:    "",
					Sleeping: true,
					Labels:   []string{"-"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPreviewOutput(tt.previews)
			assert.ElementsMatch(t, result, tt.expectedResult)
		})
	}
}

func Test_validatePreviewListOutput(t *testing.T) {
	var tests = []struct {
		name        string
		output      string
		expectedErr error
	}{
		{
			name:        "output format is yaml",
			output:      "yaml",
			expectedErr: nil,
		},
		{
			name:        "output format is json",
			output:      "json",
			expectedErr: nil,
		},
		{
			name:        "output format is not valid",
			output:      "xml",
			expectedErr: errInvalidOutput,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePreviewListOutput(tt.output)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func Test_getPreviewDefaultOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    previewOutput
		expected string
	}{
		{
			name: "preview with no labels",
			input: previewOutput{
				Name:     "my-preview",
				Scope:    "personal",
				Sleeping: false,
			},
			expected: "my-preview\tpersonal\tfalse\t-\n",
		},
		{
			name: "preview with labels",
			input: previewOutput{
				Name:     "my-preview",
				Scope:    "personal",
				Sleeping: false,
				Labels:   []string{"one", "two"},
			},
			expected: "my-preview\tpersonal\tfalse\tone, two\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPreviewDefaultOutput(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_displayListPreviews(t *testing.T) {
	tests := []struct {
		name   string
		format string
		input  []previewOutput

		expectedOutput string
	}{
		{
			name:           "empty list ",
			format:         "",
			expectedOutput: "There are no previews\n",
		},
		{
			name:           "empty list ",
			format:         "json",
			expectedOutput: "[]\n",
		},
		{
			name:           "empty list ",
			format:         "json",
			expectedOutput: "[]\n",
		},
		{
			name:   "list - default format",
			format: "",
			input: []previewOutput{
				{
					Name:     "test",
					Scope:    "personal",
					Sleeping: true,
					Labels:   []string{"test", "okteto"},
				},
				{
					Name:     "test2",
					Scope:    "global",
					Sleeping: true,
				},
			},
			expectedOutput: `Name   Scope     Sleeping  Labels
test   personal  true      test, okteto
test2  global    true      -
`,
		},
		{
			name:   "list - json format",
			format: "json",
			input: []previewOutput{
				{
					Name:     "test",
					Scope:    "personal",
					Sleeping: true,
					Labels:   []string{"test", "okteto"},
				},
				{
					Name:     "test2",
					Scope:    "global",
					Sleeping: true,
				},
			},
			expectedOutput: "[\n {\n  \"name\": \"test\",\n  \"scope\": \"personal\",\n  \"sleeping\": true,\n  \"labels\": [\n   \"test\",\n   \"okteto\"\n  ]\n },\n {\n  \"name\": \"test2\",\n  \"scope\": \"global\",\n  \"sleeping\": true,\n  \"labels\": null\n }\n]\n",
		},
		{
			name:   "list - yaml format",
			format: "yaml",
			input: []previewOutput{
				{
					Name:     "test",
					Scope:    "personal",
					Sleeping: true,
					Labels:   []string{"test", "okteto"},
				},
				{
					Name:     "test2",
					Scope:    "global",
					Sleeping: true,
				},
			},
			expectedOutput: "- name: test\n  scope: personal\n  sleeping: true\n  labels:\n  - test\n  - okteto\n- name: test2\n  scope: global\n  sleeping: true\n  labels: []\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			initialStdout := os.Stdout
			r, w, _ := os.Pipe()

			// replace Stdout for tests
			os.Stdout = w

			err := displayListPreviews(tt.input, tt.format)
			assert.NoError(t, err)

			w.Close()
			out, err := io.ReadAll(r)
			assert.NoError(t, err)

			// return back to initial
			os.Stdout = initialStdout

			assert.Equal(t, tt.expectedOutput, string(out))
		})
	}

}

func Test_newListPreviewCommand(t *testing.T) {

	tests := []struct {
		name     string
		okClient types.OktetoInterface
		flags    *listFlags
		expected *listPreviewCommand
	}{
		{
			name:     "empty input",
			expected: &listPreviewCommand{},
		},
		{
			name:     "with input",
			okClient: client.NewFakeOktetoClient(),
			flags: &listFlags{
				labels: []string{"test", "okteto"},
				output: "json",
			},
			expected: &listPreviewCommand{
				flags: &listFlags{
					labels: []string{"test", "okteto"},
					output: "json",
				},
				okClient: &client.FakeOktetoClient{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newListPreviewCommand(tt.okClient, tt.flags)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_run(t *testing.T) {

	tests := []struct {
		name      string
		cmd       *listPreviewCommand
		expectErr error
	}{
		{
			name: "invalid list output format",
			cmd: &listPreviewCommand{
				flags: &listFlags{
					output: "xml",
				},
			},
			expectErr: errInvalidOutput,
		},
		{
			name: "okClient Previews list returns error",
			cmd: &listPreviewCommand{
				flags: &listFlags{},
				okClient: &client.FakeOktetoClient{
					Preview: client.NewFakePreviewClient(
						&client.FakePreviewResponse{
							ErrList: assert.AnError,
						},
					),
				},
			},
			expectErr: assert.AnError,
		},
		{
			name: "okClient Previews list returns user error",
			cmd: &listPreviewCommand{
				flags: &listFlags{},
				okClient: &client.FakeOktetoClient{
					Preview: client.NewFakePreviewClient(
						&client.FakePreviewResponse{
							ErrList: oktetoErrors.UserError{},
						},
					),
				},
			},
			expectErr: oktetoErrors.UserError{},
		},
		{
			name: "okClient Previews list returns empty list",
			cmd: &listPreviewCommand{
				flags: &listFlags{},
				okClient: &client.FakeOktetoClient{
					Preview: client.NewFakePreviewClient(
						&client.FakePreviewResponse{
							PreviewList: []types.Preview{},
						},
					),
				},
			},
			expectErr: nil,
		},
		{
			name: "okClient Previews list returns list",
			cmd: &listPreviewCommand{
				flags: &listFlags{},
				okClient: &client.FakeOktetoClient{
					Preview: client.NewFakePreviewClient(
						&client.FakePreviewResponse{
							PreviewList: []types.Preview{
								{
									ID: "test",
								},
							},
						},
					),
				},
			},
			expectErr: nil,
		},
		{
			name: "okClient Previews list returns list with output json",
			cmd: &listPreviewCommand{
				flags: &listFlags{
					output: "json",
				},
				okClient: &client.FakeOktetoClient{
					Preview: client.NewFakePreviewClient(
						&client.FakePreviewResponse{
							PreviewList: []types.Preview{
								{
									ID: "test",
								},
							},
						},
					),
				},
			},
			expectErr: nil,
		},
		{
			name: "okClient Previews list returns list with output yaml",
			cmd: &listPreviewCommand{
				flags: &listFlags{
					output: "yaml",
				},
				okClient: &client.FakeOktetoClient{
					Preview: client.NewFakePreviewClient(
						&client.FakePreviewResponse{
							PreviewList: []types.Preview{
								{
									ID: "test",
								},
							},
						},
					),
				},
			},
			expectErr: nil,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			ctx := context.TODO()
			got := tt.cmd.run(ctx)

			assert.ErrorIs(t, got, tt.expectErr)
		})
	}

}
