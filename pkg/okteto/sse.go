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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/stream"
)

var (
	// gitDeployUrlTemplate (baseURL, namespace, dev environment name, action name)
	gitDeployUrlTemplate = "%s/sse/logs/%s/gitdeploy/%s?action=%s"
)

type sseClient struct {
	client *http.Client
}

func newSSEClient(httpClient *http.Client) *sseClient {
	return &sseClient{
		client: httpClient,
	}
}

type pipelineLogFormat oktetoLog.JSONLogFormat

// StreamPipelineLogs retrieves logs from the pipeline provided and prints them, returns error
func (c *sseClient) StreamPipelineLogs(ctx context.Context, name, namespace, actionName string) error {
	streamURL := fmt.Sprintf(gitDeployUrlTemplate, Context().Name, namespace, name, actionName)
	url, err := url.Parse(streamURL)
	if err != nil {
		return err
	}
	return stream.GetLogsFromURL(ctx, c.client, url.String(), printPipelineLog)
}

// printPipelineLog prints a line with the Message unmarshalled from line
func printPipelineLog(line string) {
	pipelineLogList := []pipelineLogFormat{}
	json.Unmarshal([]byte(line), &pipelineLogList)
	for _, pLog := range pipelineLogList {
		// stop when the event log is in stage done and message is EOF
		if pLog.Stage == "done" && pLog.Message == "EOF" {
			return
		}
		fmt.Println(pLog.Message)
	}

	pLog := pipelineLogFormat{}
	json.Unmarshal([]byte(line), &pLog)
	// stop when the event log is in stage done and message is EOF
	if pLog.Stage == "done" && pLog.Message == "EOF" {
		return
	}
	fmt.Println(pLog.Message)
}
