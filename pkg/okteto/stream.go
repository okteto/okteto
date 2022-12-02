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
	// destroyAllUrlTempleate (baseURL, namespace)
	destroyAllUrlTempleate = "%s/sse/logs/%s/destroy-all"
)

type streamClient struct {
	client *http.Client
}

func newStreamClient(httpClient *http.Client) *streamClient {
	return &streamClient{
		client: httpClient,
	}
}

type pipelineLogFormat oktetoLog.JSONLogFormat

type destroyAllLogFormat struct {
	Line string `json:"line"`
}

// PipelineLogs retrieves logs from the pipeline provided and prints them, returns error
func (c *streamClient) PipelineLogs(ctx context.Context, name, namespace, actionName string) error {
	streamURL := fmt.Sprintf(gitDeployUrlTemplate, Context().Name, namespace, name, actionName)
	url, err := url.Parse(streamURL)
	if err != nil {
		return err
	}
	return stream.GetLogsFromURL(ctx, c.client, url.String(), handlerPipelineLogLine)
}

// handlerPipelineLog prints a line with the Message unmarshalled from line
// returns true when stream has to stop
func handlerPipelineLogLine(line string) bool {
	pipelineLogList := []pipelineLogFormat{}
	if err := json.Unmarshal([]byte(line), &pipelineLogList); err != nil {
		// if not slice, try to unmarshall to log format
		pLog := pipelineLogFormat{}
		if err := json.Unmarshal([]byte(line), &pLog); err != nil {
			oktetoLog.Infof("error unmarshalling pipelineLog: %v", err)
		}
		// stop when the event log is in stage done and message is EOF
		if pLog.Stage == "done" && pLog.Message == "EOF" {
			return true
		}
		oktetoLog.Println(pLog.Message)
		return false
	}
	for _, pLog := range pipelineLogList {
		// stop when the event log is in stage done and message is EOF
		if pLog.Stage == "done" && pLog.Message == "EOF" {
			return true
		}
		oktetoLog.Println(pLog.Message)
	}
	return false
}

// DestroyAllLogs retrieves logs from the pipeline provided and prints them, returns error
func (c *streamClient) DestroyAllLogs(ctx context.Context, namespace string) error {
	streamURL := fmt.Sprintf(destroyAllUrlTempleate, Context().Name, Context().Namespace)
	url, err := url.Parse(streamURL)
	if err != nil {
		return err
	}
	return stream.GetLogsFromURL(ctx, c.client, url.String(), handlerDestroyAllLog)
}

// handlerDestroyAllLog prints a line with the Message unmarshalled from line
func handlerDestroyAllLog(line string) bool {
	destroyAllLogList := []destroyAllLogFormat{}
	if err := json.Unmarshal([]byte(line), &destroyAllLogList); err != nil {
		dLog := destroyAllLogFormat{}
		if err := json.Unmarshal([]byte(line), &dLog); err != nil {
			oktetoLog.Infof("error unmarshalling destroyAllLogFormat: %v", err)
			return false
		}
		// skip when the event log is in stage done and message is EOF
		if dLog.Line == "Done" {
			return true
		}
		oktetoLog.Println(dLog.Line)
		return false
	}
	for _, dLog := range destroyAllLogList {
		// skip when the event log is in stage done and message is EOF
		if dLog.Line == "Done" {
			return true
		}
		oktetoLog.Println(dLog.Line)
	}
	return false
}
