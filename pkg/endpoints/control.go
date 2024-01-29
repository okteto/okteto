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

package endpoints

import (
	"context"

	"github.com/okteto/okteto/pkg/okteto"
)

type APIControl struct {
	OktetoClient *okteto.Client
}

func NewEndpointControl(c *okteto.Client) *APIControl {
	return &APIControl{
		OktetoClient: c,
	}
}

func (c *APIControl) List(ctx context.Context, ns string, labelSelector string) ([]string, error) {
	return c.OktetoClient.Endpoint().List(ctx, ns, labelSelector)
}
