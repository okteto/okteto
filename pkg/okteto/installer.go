package okteto

import (
	"context"
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"github.com/shurcooL/graphql"
)

//InstallerJob represents an installer job
type InstallerJob struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// GetInstallerJob gets a installer job given its name
func (c *OktetoClient) GetInstallerJob(ctx context.Context, name, namespace string) (*InstallerJob, error) {
	var query struct {
		InstallerJob struct {
			Id     graphql.String
			Name   graphql.String
			Status graphql.String
		} `graphql:"installerJob(name: $name, space: $space)"`
	}

	variables := map[string]interface{}{
		"name":  graphql.String(name),
		"space": graphql.String(namespace),
	}

	err := c.client.Query(ctx, &query, variables)
	if err != nil {
		return nil, translateAPIErr(err)
	}

	installerJob := &InstallerJob{
		ID:     string(query.InstallerJob.Id),
		Name:   string(query.InstallerJob.Name),
		Status: string(query.InstallerJob.Status),
	}
	return installerJob, nil
}

func WaitforInstallerJobToFinish(ctx context.Context, name, jobName, namespace string, timeout time.Duration) error {
	t := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	oktetoClient, err := NewOktetoClient()
	if err != nil {
		return err
	}
	for {
		select {
		case <-to.C:
			return fmt.Errorf("installer job '%s' didn't finish after %s", name, timeout.String())
		case <-t.C:
			job, err := oktetoClient.GetInstallerJob(ctx, jobName, namespace)
			if err != nil {
				return fmt.Errorf("failed to get installer job '%s': %s", name, err)
			}

			switch job.Status {
			case "progressing", "queued":
				log.Infof("installer job '%s' is '%s'", name, job.Status)
			case "error":
				return fmt.Errorf("installer job '%s' failed", name)
			default:
				return nil
			}
		}
	}
}
