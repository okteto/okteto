package okteto

import (
	"context"
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"github.com/shurcooL/graphql"
)

// ActionBody top body answer
type ActionBody struct {
	Action Action `json:"action"`
}

//Action represents an action
type Action struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// GetAction gets a installer job given its name
func (c *OktetoClient) GetAction(ctx context.Context, name string) (*Action, error) {
	namespace := Context().Namespace
	var query struct {
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

	err := c.client.Query(ctx, &query, variables)
	if err != nil {
		return nil, translateAPIErr(err)
	}
	action := &Action{
		ID:     string(query.Action.Id),
		Name:   string(query.Action.Name),
		Status: string(query.Action.Status),
	}

	return action, nil
}

func (c *OktetoClient) WaitForActionToFinish(ctx context.Context, name string, timeout time.Duration) error {
	t := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)

	for {
		select {
		case <-to.C:
			return fmt.Errorf("action '%s' didn't finish after %s", name, timeout.String())
		case <-t.C:
			a, err := c.GetAction(ctx, name)
			if err != nil {
				return fmt.Errorf("failed to get action '%s': %s", name, err)
			}

			log.Infof("action '%s' is '%s'", name, a.Status)
			switch a.Status {
			case "progressing", "queued":
				continue
			case "error":
				return fmt.Errorf("action '%s' failed", name)
			default:
				return nil
			}
		}
	}
}
