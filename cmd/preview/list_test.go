package preview

import (
	"context"
	"fmt"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_listPreview(t *testing.T) {
	ctx := context.Background()
	opts := ListFlags{}
	var tests = []struct {
		name           string
		previews       []types.Preview
		expectedResult []previewOutput
		expectedErr    error
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
		{
			name:           "error retrieving preview list",
			previews:       nil,
			expectedResult: nil,
			expectedErr:    fmt.Errorf("error retrieving previews"),
		},
		{
			name:           "user error retrieving preview list",
			previews:       nil,
			expectedResult: nil,
			expectedErr: oktetoErrors.UserError{
				E:    fmt.Errorf("error retrieving previews"),
				Hint: "please try again",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Name:     "test",
						Token:    "test",
						IsOkteto: true,
						UserID:   "1",
					},
				},
				CurrentContext: "test",
			}
			usr := &types.User{
				Token: "test",
			}
			fakeOktetoClient := &client.FakeOktetoClient{
				Preview: client.NewFakePreviewClient(&client.FakePreviewResponse{PreviewList: tt.previews, ErrList: tt.expectedErr}),
				Users:   client.NewFakeUsersClient(usr),
			}
			result, err := getPreviewOutput(ctx, opts, fakeOktetoClient)
			if tt.expectedErr != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, result, tt.expectedResult)
			}
		})
	}
}

func Test_PreviewListOutputValidation(t *testing.T) {
	var tests = []struct {
		name        string
		output      ListFlags
		expectedErr error
	}{
		{
			name: "output format is yaml",
			output: ListFlags{
				output: "yaml",
			},
			expectedErr: nil,
		},
		{
			name: "output format is json",
			output: ListFlags{
				output: "json",
			},
			expectedErr: nil,
		},
		{
			name: "output format is not valid",
			output: ListFlags{
				output: "xml",
			},
			expectedErr: fmt.Errorf("output format is not accepted. Value must be one of: ['json', 'yaml']"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOutput(tt.output.output)
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
