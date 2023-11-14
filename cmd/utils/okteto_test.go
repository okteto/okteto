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

package utils

import (
	"context"
	"fmt"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_createContext(t *testing.T) {
	ctx := context.Background()

	var tests = []struct {
		err            error
		name           string
		namespaces     []types.Namespace
		previews       []types.Preview
		expectedErr    bool
		expectedAccess bool
	}{
		{
			name: "namespaceFound",
			namespaces: []types.Namespace{
				{
					ID: "test",
				},
			},
			expectedAccess: true,
		},
		{
			name:        "err",
			err:         fmt.Errorf("could not connect with okteto client"),
			expectedErr: true,
		},
		{
			name: "namespaceNotFound",
			namespaces: []types.Namespace{
				{
					ID: "not-found",
				},
			},
			expectedAccess: false,
		},
		{
			name: "previewFound",
			previews: []types.Preview{
				{
					ID: "test",
				},
			},
			expectedAccess: true,
		},
		{
			name: "previewFound",
			previews: []types.Preview{
				{
					ID: "not-found",
				},
			},
			expectedAccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			fakeClient := client.NewFakeOktetoClient()
			fakeClient.Namespace = client.NewFakeNamespaceClient(tt.namespaces, tt.err)
			fakeClient.Preview = client.NewFakePreviewClient(&client.FakePreviewResponse{PreviewList: tt.previews, ErrList: tt.err})
			hasAccess, err := HasAccessToOktetoClusterNamespace(ctx, "test", fakeClient)

			assert.Equal(t, tt.expectedErr, err != nil)
			assert.Equal(t, tt.expectedAccess, hasAccess)
		})
	}
}
