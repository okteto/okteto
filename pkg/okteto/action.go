package okteto

import (
	"context"
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/log"
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
func GetAction(ctx context.Context, name, namespace string) (*Action, error) {
	q := fmt.Sprintf(`query{
		action(name: "%s", space: "%s"){
			id,name,status
		},
	}`, name, namespace)

	var body ActionBody
	if err := query(ctx, q, &body); err != nil {
		return nil, err
	}

	return &body.Action, nil
}

func WaitForActionToFinish(ctx context.Context, name, namespace string, timeout time.Duration) error {
	t := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)

	for {
		select {
		case <-to.C:
			return fmt.Errorf("action '%s' didn't finish after %s", name, timeout.String())
		case <-t.C:
			a, err := GetAction(ctx, name, namespace)
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
