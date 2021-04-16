package up

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/exec"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/ssh"
)

func (up *upContext) cleanCommand(ctx context.Context) {
	in := strings.NewReader("\n")
	var out bytes.Buffer

	cmd := "cat /var/okteto/bin/version.txt; cat /proc/sys/fs/inotify/max_user_watches; /var/okteto/bin/clean >/dev/null 2>&1"

	err := exec.Exec(
		ctx,
		up.Client,
		up.RestConfig,
		up.Dev.Namespace,
		up.Pod.Name,
		up.Dev.Container,
		false,
		in,
		&out,
		os.Stderr,
		[]string{"sh", "-c", cmd},
	)

	if err != nil {
		log.Infof("failed to clean session: %s", err)
	}

	up.cleaned <- out.String()
}

func (up *upContext) runCommand(ctx context.Context) error {
	log.Infof("starting remote command")
	if err := config.UpdateStateFile(up.Dev, config.Ready); err != nil {
		return err
	}

	if up.Dev.RemoteModeEnabled() {
		return ssh.Exec(ctx, up.Dev.Interface, up.Dev.RemotePort, true, os.Stdin, os.Stdout, os.Stderr, up.Dev.Command.Values)
	}

	return exec.Exec(
		ctx,
		up.Client,
		up.RestConfig,
		up.Dev.Namespace,
		up.Pod.Name,
		up.Dev.Container,
		true,
		os.Stdin,
		os.Stdout,
		os.Stderr,
		up.Dev.Command.Values,
	)
}

func (up *upContext) checkOktetoStartError(ctx context.Context, msg string) error {
	userID := pods.GetDevPodUserID(ctx, up.Dev, up.Client)
	if up.Dev.PersistentVolumeEnabled() {
		if userID != -1 && userID != *up.Dev.SecurityContext.RunAsUser {
			return errors.UserError{
				E: fmt.Errorf("User %d doesn't have write permissions for the %s directory", userID, up.Dev.MountPath),
				Hint: fmt.Sprintf(`Set 'securityContext.runAsUser: %d' in your okteto manifest.
    After that, run 'okteto down -v' to reset your development container and run 'okteto up' again`, userID),
			}
		}
	}

	if len(up.Dev.Secrets) > 0 {
		return errors.UserError{
			E: fmt.Errorf(msg),
			Hint: fmt.Sprintf(`Check your development container logs for errors: 'kubectl logs %s',
    Check that your container can write to the destination path of your secrets.
    Run 'okteto down -v' to reset your development container and try again`, up.Pod.Name),
		}
	}
	return errors.UserError{
		E: fmt.Errorf(msg),
		Hint: fmt.Sprintf(`Check your development container logs for errors: 'kubectl logs %s'.
    Run 'okteto down -v' to reset your development container and try again`, up.Pod.Name),
	}
}
