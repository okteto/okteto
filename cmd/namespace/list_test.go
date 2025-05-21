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

package namespace

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_listNamespace(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		err               error
		name              string
		currentNamespaces []types.Namespace
	}{
		{
			name: "List all ns",
			currentNamespaces: []types.Namespace{
				{
					ID: "test",
				},
				{
					ID: "test-1",
				},
			},
		},
		{
			name:              "error retrieving ns",
			currentNamespaces: nil,
			err:               fmt.Errorf("error retrieving ns"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
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
				Namespace: client.NewFakeNamespaceClient(tt.currentNamespaces, tt.err),
				Users:     client.NewFakeUsersClient(usr),
			}
			nsCmd := &Command{
				okClient: fakeOktetoClient,
				ctxCmd:   newFakeContextCommand(fakeOktetoClient, usr),
			}
			err := nsCmd.executeListNamespaces(ctx, "")
			if tt.err != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

		})
	}
}

func Test_validateNamespaceListOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		expectedErr error
	}{
		{
			name:   "yaml output",
			output: "yaml",
		},
		{
			name:   "json output",
			output: "json",
		},
		{
			name:        "invalid output",
			output:      "xml",
			expectedErr: errInvalidOutput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNamespaceListOutput(tt.output)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func Test_displayListNamespaces(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		input          []namespaceOutput
		expectedOutput string
	}{
		{
			name:           "empty default",
			format:         "",
			expectedOutput: "There are no namespaces\n",
		},
		{
			name:           "empty json",
			format:         "json",
			expectedOutput: "[]\n",
		},
		{
			name:   "default format",
			format: "",
			input: []namespaceOutput{
				{Namespace: "test", Status: "Active"},
				{Namespace: "test2", Status: "Sleeping"},
			},
			expectedOutput: "Namespace  Status\ntest *     Active\ntest2      Sleeping\n",
		},
		{
			name:   "json format",
			format: "json",
			input: []namespaceOutput{
				{Namespace: "test", Status: "Active"},
				{Namespace: "test2", Status: "Sleeping"},
			},
			expectedOutput: "[\n {\n  \"namespace\": \"test\",\n  \"status\": \"Active\"\n },\n {\n  \"namespace\": \"test2\",\n  \"status\": \"Sleeping\"\n }\n]\n",
		},
		{
			name:   "yaml format",
			format: "yaml",
			input: []namespaceOutput{
				{Namespace: "test", Status: "Active"},
				{Namespace: "test2", Status: "Sleeping"},
			},
			expectedOutput: "- namespace: test\n  status: Active\n- namespace: test2\n  status: Sleeping\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"test": {
						Namespace: "test",
						IsOkteto:  true,
					},
				},
				CurrentContext: "test",
			}
			r, w, _ := os.Pipe()
			initialStdout := os.Stdout
			os.Stdout = w

			err := displayListNamespaces(tt.input, tt.format)
			assert.NoError(t, err)

			w.Close()
			out, _ := io.ReadAll(r)
			os.Stdout = initialStdout

			assert.Equal(t, tt.expectedOutput, string(out))
		})
	}
}
