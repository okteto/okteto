package okteto

import (
	"context"
	"fmt"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
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
	queryStruct := getActionQueryStruct{}
	variables := map[string]interface{}{
		"name":  graphql.String(name),
		"space": graphql.String(namespace),
	}

	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, err
	}
	action := &types.Action{
		ID:     string(queryStruct.Action.Id),
		Name:   string(queryStruct.Action.Name),
		Status: string(queryStruct.Action.Status),
	}

	return action, nil
}

func (c *pipelineClient) WaitForActionToFinish(ctx context.Context, pipelineName, namespace, actionName string, timeout time.Duration) error {
	t := c.tickerWithTimeoutProvider(1*time.Second, timeout)
	for {
		select {
		case <-t.TimeoutTick():
			oktetoLog.Infof("action '%s' didn't finish after %s", actionName, timeout.String())
			return pipelineTimeoutError{
				pipelineName: actionName,
				timeout:      timeout,
			}
		case <-t.TickerTick():
			a, err := c.GetAction(ctx, actionName, namespace)
			if err != nil {
				oktetoLog.Infof("action '%s' failed", actionName)
				return fmt.Errorf("pipeline '%s' failed: %w", pipelineName, err)
			}

			oktetoLog.Infof("action '%s' is '%s'", actionName, a.Status)
			switch a.Status {
			case "progressing", "queued":
				continue
			case "error", "destroy-error":
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
	t := c.tickerWithTimeoutProvider(1*time.Second, timeout)

	for {
		select {
		case <-t.TimeoutTick():
			oktetoLog.Infof("action '%s' didn't progress after %s", actionName, timeout.String())
			return pipelineTimeoutError{
				pipelineName: actionName,
				timeout:      timeout,
			}
		case <-t.TickerTick():
			a, err := c.GetAction(ctx, actionName, namespace)
			if err != nil {
				oktetoLog.Infof("action '%s' failed", actionName)
				return fmt.Errorf("pipeline '%s' failed: %w", pipelineName, err)
			}

			oktetoLog.Infof("action '%s' is '%s'", actionName, a.Status)
			switch a.Status {
			case "progressing":
				return nil
			case "queued":
				continue
			case "error", "destroy-error":
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
