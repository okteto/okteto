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

package okteto

import (
	"context"
	"fmt"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

const (
	progressingStatus string = "progressing"
	queuedStatus      string = "queued"
	errorStatus       string = "error"
	destroyErrStatus  string = "destroy-error"

	tickerInterval time.Duration = 1 * time.Second
)

type getActionQueryStruct struct {
	Action actionStruct `graphql:"action(name: $name, space: $space)"`
}

type actionStruct struct {
	Id     graphql.String
	Name   graphql.String
	Status graphql.String
}

// GetAction gets a installer job given its name
func (c *pipelineClient) GetAction(ctx context.Context, name, namespace string) (*types.Action, error) {
	oktetoLog.Infof("getting action '%s' on %s", name, namespace)
	queryStruct := getActionQueryStruct{}
	variables := map[string]interface{}{
		"name":  graphql.String(name),
		"space": graphql.String(namespace),
	}

	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, fmt.Errorf("failed to get action '%s': %w", name, err)
	}
	action := &types.Action{
		ID:     string(queryStruct.Action.Id),
		Name:   string(queryStruct.Action.Name),
		Status: string(queryStruct.Action.Status),
	}

	return action, nil
}

func (c *pipelineClient) WaitForActionToFinish(ctx context.Context, pipelineName, namespace, actionName string, timeout time.Duration) error {
	oktetoLog.Infof("waiting for action '%s' to finish", actionName)
	timeoutTimer := c.provideTimer(timeout)
	ticker := c.provideTicker(tickerInterval)
	for {
		select {
		case <-timeoutTimer.C:
			oktetoLog.Infof("action '%s' didn't finish after %s", actionName, timeout.String())
			return pipelineTimeoutError{
				pipelineName: actionName,
				timeout:      timeout,
			}
		case <-ticker.C:
			a, err := c.GetAction(ctx, actionName, namespace)
			if err != nil {
				oktetoLog.Infof("action '%s' failed", actionName)
				return fmt.Errorf("pipeline '%s' failed: %w", pipelineName, err)
			}

			oktetoLog.Infof("action '%s' is '%s'", actionName, a.Status)
			switch a.Status {
			case progressingStatus, queuedStatus:
				continue
			case errorStatus, destroyErrStatus:
				oktetoLog.Infof("action '%s' failed", actionName)
				return pipelineFailedError{
					pipelineName: pipelineName,
				}
			default:
				return nil
			}
		}
	}
}

func (c *pipelineClient) WaitForActionProgressing(ctx context.Context, pipelineName, namespace, actionName string, timeout time.Duration) error {
	oktetoLog.Infof("waiting for action '%s' to start", actionName)
	timeoutTimer := c.provideTimer(timeout)
	ticker := c.provideTicker(tickerInterval)
	for {
		select {
		case <-timeoutTimer.C:
			oktetoLog.Infof("action '%s' didn't progress after %s", actionName, timeout.String())
			return pipelineTimeoutError{
				pipelineName: actionName,
				timeout:      timeout,
			}
		case <-ticker.C:
			a, err := c.GetAction(ctx, actionName, namespace)
			if err != nil {
				oktetoLog.Infof("action '%s' failed", actionName)
				return fmt.Errorf("pipeline '%s' failed: %w", pipelineName, err)
			}

			oktetoLog.Infof("action '%s' is '%s'", actionName, a.Status)
			switch a.Status {
			case progressingStatus:
				return nil
			case queuedStatus:
				continue
			case errorStatus, destroyErrStatus:
				oktetoLog.Infof("action '%s' failed", actionName)
				return pipelineFailedError{
					pipelineName: pipelineName,
				}
			default:
				return nil
			}
		}
	}
}
