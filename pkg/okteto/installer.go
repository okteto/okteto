package okteto

import (
	"context"
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/log"
)

// InstallerJobBody top body answer
type InstallerJobBody struct {
	InstallerJob InstallerJob `json:"installerJob"`
}

//InstallerJob represents an installer job
type InstallerJob struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// GetInstallerJob gets a installer job given its name
func GetInstallerJob(ctx context.Context, name, namespace string) (*InstallerJob, error) {
	q := fmt.Sprintf(`query{
		installerJob(name: "%s", space: "%s"){
			id,name,status
		},
	}`, name, namespace)

	var body InstallerJobBody
	if err := query(ctx, q, &body); err != nil {
		return nil, err
	}

	return &body.InstallerJob, nil
}

func WaitforInstallerJobToFinish(ctx context.Context, name, jobName, namespace string, timeout time.Duration) error {
	t := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)

	for {
		select {
		case <-to.C:
			return fmt.Errorf("installer job '%s' didn't finish after %s", name, timeout.String())
		case <-t.C:
			job, err := GetInstallerJob(ctx, jobName, namespace)
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
