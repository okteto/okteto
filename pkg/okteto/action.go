package okteto

import (
	"context"
	"fmt"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

// GetAction gets a installer job given its name
func (c *OktetoClient) GetAction(ctx context.Context, name string) (*types.Action, error) {
	namespace := Context().Namespace
	var queryStruct struct {
		Action struct {
			Id     graphql.String
			Name   graphql.String
			Status graphql.String
		} `graphql:"action(name: $name, space: $space)"`
	}
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

func (c *OktetoClient) WaitForActionToFinish(ctx context.Context, pipelineName, actionName string, timeout time.Duration) error {
	t := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)

	for {
		select {
		case <-to.C:
			oktetoLog.Infof("action '%s' didn't finish after %s", actionName, timeout.String())
			return fmt.Errorf("'%s' didn't finish after %s", pipelineName, timeout.String())
		case <-t.C:
			a, err := c.GetAction(ctx, actionName)
			if err != nil {
				oktetoLog.Infof("action '%s' failed", actionName)
				return fmt.Errorf("pipeline '%s' failed", pipelineName)
			}

			oktetoLog.Infof("action '%s' is '%s'", actionName, a.Status)
			switch a.Status {
			case "progressing", "queued":
				continue
			case "error":
				oktetoLog.Infof("action '%s' failed", actionName)
				return fmt.Errorf("pipeline '%s' failed", pipelineName)
			default:
				return nil
			}
		}
	}
}
