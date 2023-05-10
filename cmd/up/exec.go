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

package up

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/creack/pty"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	k8sExec "github.com/okteto/okteto/pkg/k8s/exec"
	"github.com/okteto/okteto/pkg/k8s/pods"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/ssh"
	"k8s.io/client-go/kubernetes"
)

type devExecutor interface {
	runCommand(ctx context.Context, cmd []string) error
}

type hybridExecutor struct {
	workdir string
	envs    []string
}

func (he *hybridExecutor) runCommand(_ context.Context, cmd []string) error {
	c := exec.Command("bash", "-c", strings.Join(cmd, " "))
	c.Dir = he.workdir
	c.Env = append(os.Environ(), he.envs...)

	f, err := pty.Start(c)
	if err != nil {
		return err
	}

	io.Copy(os.Stdout, f)
	return nil
}

type syncExecutor struct {
	iface      string
	remotePort int
}

func (se *syncExecutor) runCommand(ctx context.Context, cmd []string) error {
	return ssh.Exec(ctx, se.iface, se.remotePort, true, os.Stdin, os.Stdout, os.Stderr, cmd)
}

func newExecutorByDevMode(ctx context.Context, up *upContext) (devExecutor, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	if up.Dev.IsHybridModeEnabled() {
		envs, err := getEnvsFromContext(ctx, up.Dev, up.Client, up.Manifest.Name, up.Manifest.Namespace)
		if err != nil {
			return nil, err
		}

		return &hybridExecutor{
			workdir: filepath.Join(wd, up.Dev.Workdir),
			envs:    envs,
		}, nil
	}
	return &syncExecutor{
		iface:      up.Dev.Interface,
		remotePort: up.Dev.RemotePort,
	}, nil
}

func getEnvsFromContext(ctx context.Context, dev *model.Dev, c kubernetes.Interface, name string, namespace string) ([]string, error) {
	var envs []string

	// get config map variables
	devCmap, err := configmaps.Get(ctx, pipeline.TranslatePipelineName(name), namespace, c)
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return nil, err
	}

	if devCmap != nil && devCmap.Data != nil {
		if devVariables, ok := devCmap.Data[constants.OktetoConfigMapVariablesField]; ok {
			envInConfigMap := []map[string]string{}
			decodedEnvs, err := base64.StdEncoding.DecodeString(devVariables)
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(decodedEnvs, &envInConfigMap)
			if err != nil {
				return nil, err
			}
			for _, envKeyValue := range envInConfigMap {
				for k, v := range envKeyValue {
					envs = append(envs, fmt.Sprintf("%s=%s", k, v))
				}
			}
		}
	}

	// get envs from secrets
	if okteto.IsOkteto() {
		c, err := okteto.NewOktetoClient()
		if err != nil {
			return nil, err
		}

		secrets, err := c.User().GetUserSecrets(ctx)
		if err != nil {
			return nil, err
		}

		for _, s := range secrets {
			envs = append(envs, fmt.Sprintf("%s=%s", s.Name, s.Value))
		}
	}

	// get image envs
	app, err := apps.Get(ctx, dev, dev.Namespace, c)
	if err != nil {
		return nil, err
	}

	imageTag := app.PodSpec().Containers[0].Image
	reg := registry.NewOktetoRegistry(okteto.Config{})
	imageMetadata, err := reg.GetImageMetadata(imageTag)
	if err != nil {
		return nil, err
	}

	for _, env := range imageMetadata.Envs {
		if strings.HasPrefix(env, "PATH=") {
			continue
		}
		envs = append(envs, env)

	}
	return envs, nil
}

func (up *upContext) cleanCommand(ctx context.Context) {
	in := strings.NewReader("\n")
	var out bytes.Buffer

	cmd := "cat /var/okteto/bin/version.txt; cat /proc/sys/fs/inotify/max_user_watches; /var/okteto/bin/clean >/dev/null 2>&1"

	err := k8sExec.Exec(
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
		oktetoLog.Infof("failed to clean session: %s", err)
	}

	up.cleaned <- out.String()
}

func (up *upContext) runCommand(ctx context.Context, cmd []string) error {
	oktetoLog.Infof("starting remote command")
	if err := config.UpdateStateFile(up.Dev.Name, up.Dev.Namespace, config.Ready); err != nil {
		return err
	}

	if up.Dev.RemoteModeEnabled() {
		executor, err := newExecutorByDevMode(ctx, up)
		if err != nil {
			return err
		}
		return executor.runCommand(ctx, cmd)
	}

	return k8sExec.Exec(
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

	devApp := app.DevClone()
	if err := devApp.Refresh(ctx, up.Client); err != nil {
		return err
	}
	pod, err := devApp.GetRunningPod(ctx, up.Client)
	if err != nil {
		return err
	}

	userID := pods.GetPodUserID(ctx, pod.Name, up.Dev.Container, up.Dev.Namespace, up.Client)
	if up.Dev.PersistentVolumeEnabled() {
		if userID != -1 && userID != *up.Dev.SecurityContext.RunAsUser {
			return oktetoErrors.UserError{
				E: fmt.Errorf("User %d doesn't have write permissions for synchronization paths", userID),
				Hint: fmt.Sprintf(`Set 'securityContext.runAsUser: %d' in your okteto manifest.
	After that, run '%s' to reset your development container and run 'okteto up' again`, userID, utils.GetDownCommand(up.Options.ManifestPathFlag)),
			}
		}
	}

	if len(up.Dev.Secrets) > 0 {
		return oktetoErrors.UserError{
			E: fmt.Errorf(msg),
			Hint: fmt.Sprintf(`Check your development container logs for errors: 'kubectl logs %s',
	Check that your container can write to the destination path of your secrets.
	Run '%s' to reset your development container and try again`, up.Pod.Name, utils.GetDownCommand(up.Options.ManifestPathFlag)),
		}
	}
	return oktetoErrors.UserError{
		E: fmt.Errorf(msg),
		Hint: fmt.Sprintf(`Check your development container logs for errors: 'kubectl logs %s'.
    Run '%s' to reset your development container and try again`, up.Pod.Name, utils.GetDownCommand(up.Options.ManifestPathFlag)),
	}
}
