// Copyright 2021 The Okteto Authors
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

package up

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/exec"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/ssh"
)

func (up *upContext) cleanCommand(ctx context.Context) {
	in := strings.NewReader("\n")
	var out bytes.Buffer

	cmd := "cat /var/okteto/bin/version.txt; cat /proc/sys/fs/inotify/max_user_watches; [ -f /var/okteto/cloudbin/start.sh ] && echo yes || echo no; /var/okteto/bin/clean >/dev/null 2>&1"

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

func (up *upContext) runCommand(ctx context.Context, cmd []string) error {
	log.Infof("starting remote command")
	if err := config.UpdateStateFile(up.Dev, config.Ready); err != nil {
		return err
	}

	if up.Dev.RemoteModeEnabled() {
		return ssh.Exec(ctx, up.Dev.Interface, up.Dev.RemotePort, true, os.Stdin, os.Stdout, os.Stderr, cmd)
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
		cmd,
	)
}

func (up *upContext) checkOktetoStartError(ctx context.Context, msg string) error {
	app, err := apps.Get(ctx, up.Dev, up.Dev.Namespace, up.Client)
	if err != nil {
		return err
	}

	pod, err := app.GetRunningPod(ctx, up.Client)
	if err != nil {
		return err
	}

	userID := pods.GetPodUserID(ctx, pod.Name, up.Dev.Container, up.Dev.Namespace, up.Client)
	if up.Dev.PersistentVolumeEnabled() {
		if userID != -1 && userID != *up.Dev.SecurityContext.RunAsUser {
			return errors.UserError{
				E: fmt.Errorf("User %d doesn't have write permissions for synchronization paths", userID),
				Hint: fmt.Sprintf(`Set 'securityContext.runAsUser: %d' in your okteto manifest.
	After that, run '%s' to reset your development container and run 'okteto up' again`, userID, utils.GetDownCommand(up.Options.DevPath)),
			}
		}
	}

	if len(up.Dev.Secrets) > 0 {
		return errors.UserError{
			E: fmt.Errorf(msg),
			Hint: fmt.Sprintf(`Check your development container logs for errors: 'kubectl logs %s',
	Check that your container can write to the destination path of your secrets.
	Run '%s' to reset your development container and try again`, up.Pod.Name, utils.GetDownCommand(up.Options.DevPath)),
		}
	}
	return errors.UserError{
		E: fmt.Errorf(msg),
		Hint: fmt.Sprintf(`Check your development container logs for errors: 'kubectl logs %s'.
    Run '%s' to reset your development container and try again`, up.Pod.Name, utils.GetDownCommand(up.Options.DevPath)),
	}
}
