// Copyright 2020 The Okteto Authors
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

package okteto

import (
	"context"
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/errors"
)

// CreatePreviewBody top body answer
type CreatePreviewBody struct {
	PreviewEnviroment PreviewEnv `json:"createPreview" yaml:"createPreview"`
}

//Previews represents an Okteto list of spaces
type Previews struct {
	Previews []PreviewEnv `json:"previews" yaml:"previews"`
}

//PreviewEnv represents an Okteto preview environment
type PreviewEnv struct {
	ID       string `json:"id" yaml:"id"`
	Sleeping bool   `json:"sleeping" yaml:"sleeping"`
	Scope    string `json:"scope" yaml:"scope"`
}

// CreatePreview creates a preview environment
func CreatePreview(ctx context.Context, name, scope string) (string, error) {
	if err := validateNamespace(name); err != nil {
		return "", err
	}

	var body CreatePreviewBody
	q := fmt.Sprintf(`mutation {
		createPreview(name: "%s", scope: %s){
			id
		},
	}`, name, scope)

	if err := query(ctx, q, &body); err != nil {
		if strings.Contains(err.Error(), "operation-not-permitted") {
			return "", errors.UserError{E: fmt.Errorf("You are not authorized to create a global preview env."),
				Hint: "Please log in with an administrator account or use a personal preview environment"}
		}
		return "", err
	}

	return body.PreviewEnviroment.ID, nil
}

// DestroyPreview destroy a preview environment
func DestroyPreview(ctx context.Context, name string) error {
	q := fmt.Sprintf(`mutation{
		deletePreview(id: "%s"){
			id
		},
	}`, name)

	var body DeleteBody
	return query(ctx, q, &body)
}

// ListPreviews list preview environments
func ListPreviews(ctx context.Context) ([]PreviewEnv, error) {
	q := `query{
		previews{
			id,
			sleeping,
			scope,
		},
	}`

	var body Previews
	if err := query(ctx, q, &body); err != nil {
		return nil, err
	}

	return body.Previews, nil
}
