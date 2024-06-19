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
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_HasAccessToOktetoClusterNamespace(t *testing.T) {
	ctx := context.Background()

	var tests = []struct {
		errNamespace   error
		errPreview     error
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
			name:         "err",
			errNamespace: fmt.Errorf("could not connect with okteto client"),
			expectedErr:  true,
		},
		{
			name: "namespaceNotFound and preview not found",
			namespaces: []types.Namespace{
				{
					ID: "not-found",
				},
			},
			errNamespace:   oktetoErrors.ErrNamespaceNotFound,
			errPreview:     oktetoErrors.ErrNamespaceNotFound,
			expectedAccess: false,
		},
		{
			name: "previewFound",
			previews: []types.Preview{
				{
					ID: "test",
				},
			},
			errNamespace:   oktetoErrors.ErrNamespaceNotFound,
			expectedAccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			fakeClient := client.NewFakeOktetoClient()
			fakeClient.Namespace = client.NewFakeNamespaceClient(tt.namespaces, tt.errNamespace)
			fakeClient.Preview = client.NewFakePreviewClient(&client.FakePreviewResponse{PreviewList: tt.previews, ErrGetPreview: tt.errPreview})
			hasAccess, err := HasAccessToOktetoClusterNamespace(ctx, "test", fakeClient)

			assert.Equal(t, tt.expectedErr, err != nil)
			assert.Equal(t, tt.expectedAccess, hasAccess)
		})
	}
}
