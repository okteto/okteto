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
	"github.com/stretchr/testify/assert"
)

func Test_HasAccessToOktetoClusterNamespace(t *testing.T) {
	ctx := context.Background()

	var tests = []struct {
		oktetoClient *client.FakeOktetoClient
		name         string
		want         bool
		wantErr      bool
	}{
		{
			name: "namespace found, no fallback",
			oktetoClient: &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient(nil, nil),
				Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
			},
			want: true,
		},
		{
			name: "namespace query error not-found, fallback to preview found",
			oktetoClient: &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient(nil, oktetoErrors.ErrNotFound),
				Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
			},
			want: true,
		},
		{
			name: "namespace query error namespace-not-found, fallback to preview found",
			oktetoClient: &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient(nil, oktetoErrors.ErrNamespaceNotFound),
				Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
			},
			want: true,
		},
		{
			name: "namespace query error, no fallback",
			oktetoClient: &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient(nil, fmt.Errorf("error at query")),
				Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "fallback to previews, preview query namespace-not-found",
			oktetoClient: &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient(nil, oktetoErrors.ErrNotFound),
				Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{ErrGetPreview: oktetoErrors.ErrNamespaceNotFound}),
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "fallback to previews, preview query error",
			oktetoClient: &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient(nil, oktetoErrors.ErrNotFound),
				Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{ErrGetPreview: fmt.Errorf("error at query")}),
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "fallback to previews, preview found",
			oktetoClient: &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient(nil, oktetoErrors.ErrNotFound),
				Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
			},
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			hasAccess, err := HasAccessToOktetoClusterNamespace(ctx, "test", tt.oktetoClient)

			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, hasAccess)
		})
	}
}
