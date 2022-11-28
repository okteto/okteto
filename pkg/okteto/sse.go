// Copyright 2022 The Okteto Authors
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
	"net/http"
	"net/url"

	"github.com/okteto/okteto/pkg/sse"
)

type sseClient struct {
	client *http.Client
}

func newSSEClient(httpClient *http.Client) *sseClient {
	return &sseClient{
		client: httpClient,
	}
}

// StreamLogs retrieves logs from the pipeline provided and prints them, returns error
func (c *sseClient) StreamPipelineLogs(ctx context.Context, name, namespace, actionName string) error {
	streamURL := fmt.Sprintf(sse.GitDeployUrlTemplate, Context().Name, namespace, name, actionName)
	url, err := url.Parse(streamURL)
	if err != nil {
		return err
	}
	return sse.Stream(ctx, c.client, url.String())
}
