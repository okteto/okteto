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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/okteto/okteto/pkg/k8s/secrets"

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
	"github.com/okteto/okteto/pkg/types"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type hybridExecutor struct {
	workdir string
	envs    []string
}

type HybridExecCtx struct {
	Workdir         string
	Dev             *model.Dev
	Name, Namespace string
	Client          kubernetes.Interface
}

// GetCommandToExec returns the command to exec into the hybrid mode
func (he *hybridExecutor) GetCommandToExec(cmd []string) (*exec.Cmd, error) {
	var c *exec.Cmd
	if runtime.GOOS != "windows" {
		c = exec.Command(cmd[0], cmd[1:]...)
	} else {
		binary, err := expandExecutableInCurrentDirectory(cmd[0], he.workdir)
		if err != nil {
			return nil, err
		}
		c = exec.Command(binary, cmd[1:]...)
	}

	c.Env = he.envs

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	c.Dir = he.workdir

	return c, nil
}

func (he *hybridExecutor) RunCommand(cmd *exec.Cmd) error {
	return cmd.Run()
}

func expandExecutableInCurrentDirectory(args0, dir string) (string, error) {
	// Works around a restriction added in go1.19 that executables in the
	// current directory are not resolved when specifying just an executable
	// name (like e.g. "okteto")
	if !strings.ContainsRune(args0, os.PathSeparator) {
		// Check if it's in PATH
		_, err := exec.LookPath(args0)
		if err != nil {
			// Try to get the path to the current executable
			executable := filepath.Join(dir, args0)
			_, err := exec.LookPath(executable)
			if err != nil {
				return "", fmt.Errorf("could not determine the script file")
			}
			return executable, nil
		}
	}
	return args0, nil
}

type syncExecutor struct {
	iface      string
	remotePort int
}

func (se *syncExecutor) RunCommand(ctx context.Context, cmd []string) error {
	return ssh.Exec(ctx, se.iface, se.remotePort, true, os.Stdin, os.Stdout, os.Stderr, cmd)
}

func NewHybridExecutor(ctx context.Context, hybridCtx *HybridExecCtx) (*hybridExecutor, error) {
	envsGetter, err := newEnvsGetter(hybridCtx)
	if err != nil {
		return nil, err
	}

	envs, err := envsGetter.getEnvs(ctx)
	if err != nil {
		return nil, err
	}

	return &hybridExecutor{
		workdir: hybridCtx.Workdir,
		envs:    envs,
	}, nil
}

func newSyncExecutor(up *upContext) *syncExecutor {
	return &syncExecutor{
		iface:      up.Dev.Interface,
		remotePort: up.Dev.RemotePort,
	}
}

type devContainerEnvGetterInterface interface {
	getEnvsFromDevContainer(ctx context.Context, spec *apiv1.PodSpec, name, namespace string, client kubernetes.Interface) ([]string, error)
}

type configMapEnvsGetterInterface interface {
	getEnvsFromConfigMap(ctx context.Context, name string, namespace string, client kubernetes.Interface) ([]string, error)
}

type secretsEnvsGetterInterface interface {
	getEnvsFromSecrets(context.Context) ([]string, error)
}

type imageEnvsGetterInterface interface {
	getEnvsFromImage(string) ([]string, error)
}

type imageGetterInterface interface {
	GetImageMetadata(string) (registry.ImageMetadata, error)
}

type secretsGetterInterface interface {
	GetUserSecrets(context.Context) ([]types.Secret, error)
}

type devContainerEnvGetter struct{}
type configMapGetter struct{}
type secretsEnvsGetter struct {
	secretsGetter secretsGetterInterface
}
type imageEnvsGetter struct {
	imageGetter imageGetterInterface
}

type envsGetter struct {
	dev                   *model.Dev
	name, namespace       string
	client                kubernetes.Interface
	devContainerEnvGetter devContainerEnvGetterInterface
	configMapEnvsGetter   configMapEnvsGetterInterface
	secretsEnvsGetter     secretsEnvsGetterInterface
	imageEnvsGetter       imageEnvsGetterInterface
	getDefaultLocalEnvs   func() []string
}

func newEnvsGetter(hybridCtx *HybridExecCtx) (*envsGetter, error) {

	var secretsGetter secretsGetterInterface
	if okteto.IsOkteto() {
		oc, err := okteto.NewOktetoClient()
		if err != nil {
			return nil, err
		}
		secretsGetter = oc.User()
	}

	return &envsGetter{
		dev:                   hybridCtx.Dev,
		name:                  hybridCtx.Name,
		namespace:             hybridCtx.Namespace,
		client:                hybridCtx.Client,
		devContainerEnvGetter: &devContainerEnvGetter{},
		configMapEnvsGetter:   &configMapGetter{},
		secretsEnvsGetter: &secretsEnvsGetter{
			secretsGetter: secretsGetter,
		},
		imageEnvsGetter: &imageEnvsGetter{
			imageGetter: registry.NewOktetoRegistry(okteto.Config{}),
		},
		getDefaultLocalEnvs: getDefaultLocalEnvs,
	}, nil
}

func (eg *envsGetter) getEnvs(ctx context.Context) ([]string, error) {
	var envs []string

	app, err := apps.Get(ctx, eg.dev, eg.namespace, eg.client)
	if err != nil {
		return nil, err
	}

	imageEnvs, err := eg.imageEnvsGetter.getEnvsFromImage(apps.GetDevContainer(app.PodSpec(), "").Image)
	if err != nil {
		return nil, err
	}
	envs = append(envs, imageEnvs...)

	secretsEnvs, err := eg.secretsEnvsGetter.getEnvsFromSecrets(ctx)
	if err != nil {
		return nil, err
	}
	envs = append(envs, secretsEnvs...)

	configMapEnvs, err := eg.configMapEnvsGetter.getEnvsFromConfigMap(ctx, eg.name, eg.namespace, eg.client)
	if err != nil {
		return nil, err
	}
	envs = append(envs, configMapEnvs...)

	devContainerEnvs, err := eg.devContainerEnvGetter.getEnvsFromDevContainer(ctx, app.PodSpec(), "", eg.namespace, eg.client)
	if err != nil {
		return nil, err
	}
	envs = append(envs, devContainerEnvs...)

	envs = append(envs, eg.getDefaultLocalEnvs()...)

	for _, env := range eg.dev.Environment {
		envs = append(envs, fmt.Sprintf("%s=%s", env.Name, env.Value))
	}

	return envs, nil
}

func getDefaultLocalEnvs() []string {
	var envs []string

	path := os.Getenv("PATH")
	if path != "" {
		envs = append(envs, fmt.Sprintf("PATH=%s", path))
	}

	term := os.Getenv("TERM")
	if term != "" {
		envs = append(envs, fmt.Sprintf("TERM=%s", term))
	}

	return envs
}

func (d *devContainerEnvGetter) getEnvsFromDevContainer(ctx context.Context, spec *apiv1.PodSpec, name, namespace string, client kubernetes.Interface) ([]string, error) {
	var envs []string

	devContainer := apps.GetDevContainer(spec, name)

	for _, env := range devContainer.Env {
		val := env.Value
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			secret, err := secrets.Get(ctx, env.ValueFrom.SecretKeyRef.Name, namespace, client)
			if err != nil {
				return envs, fmt.Errorf("%w: the development container didn't start successfully because the kubernetes secret '%s' was not found", err, env.ValueFrom.SecretKeyRef.Name)
			}
			val = string(secret.Data[env.ValueFrom.SecretKeyRef.Key])
		}

		if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
			cm, err := configmaps.Get(ctx, env.ValueFrom.ConfigMapKeyRef.Name, namespace, client)
			if err != nil {
				return envs, fmt.Errorf("%w: the development container didn't start successfully because the kubernetes configmap '%s' was not found", err, env.ValueFrom.ConfigMapKeyRef.Name)
			}
			val = cm.Data[env.ValueFrom.ConfigMapKeyRef.Key]
		}
		envs = append(envs, fmt.Sprintf("%s=%s", env.Name, val))
	}

	return envs, nil
}

func (cmg *configMapGetter) getEnvsFromConfigMap(ctx context.Context, name string, namespace string, client kubernetes.Interface) ([]string, error) {
	var envs []string

	cmName := pipeline.TranslatePipelineName(name)
	devCmap, err := configmaps.Get(ctx, cmName, namespace, client)
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

	return envs, nil
}

func (sg *secretsEnvsGetter) getEnvsFromSecrets(ctx context.Context) ([]string, error) {
	var envs []string

	if okteto.IsOkteto() {
		secrets, err := sg.secretsGetter.GetUserSecrets(ctx)
		if err != nil {
			return nil, err
		}

		for _, s := range secrets {
			envs = append(envs, fmt.Sprintf("%s=%s", s.Name, s.Value))
		}
	}

	return envs, nil
}

func (ig *imageEnvsGetter) getEnvsFromImage(imageTag string) ([]string, error) {
	var envs []string

	imageMetadata, err := ig.imageGetter.GetImageMetadata(imageTag)
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

	k8sClient, restConfig, err := up.K8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		oktetoLog.Infof("failed to clean session: %s", err)
		return
	}

	err = k8sExec.Exec(
		ctx,
		k8sClient,
		restConfig,
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

func (up *upContext) RunCommand(ctx context.Context, cmd []string) error {
	oktetoLog.Infof("starting remote command")
	if err := config.UpdateStateFile(up.Dev.Name, up.Dev.Namespace, config.Ready); err != nil {
		return err
	}

	k8sClient, restConfig, err := up.K8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}

	if up.Dev.RemoteModeEnabled() {
		if up.Dev.IsHybridModeEnabled() {
			hybridCtx := &HybridExecCtx{
				Dev:       up.Dev,
				Name:      up.Manifest.Name,
				Namespace: up.Manifest.Namespace,
				Client:    k8sClient,
				Workdir:   up.Dev.Workdir,
			}
			executor, err := NewHybridExecutor(ctx, hybridCtx)
			if err != nil {
				return err
			}

			cmd, err := executor.GetCommandToExec(cmd)
			if err != nil {
				return err
			}

			up.hybridCommand = cmd

			return executor.RunCommand(cmd)
		} else {
			executor := newSyncExecutor(up)
			return executor.RunCommand(ctx, cmd)
		}

	}

	return k8sExec.Exec(
		ctx,
		k8sClient,
		restConfig,
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
	k8sClient, _, err := up.K8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}

	app, err := apps.Get(ctx, up.Dev, up.Dev.Namespace, k8sClient)
	if err != nil {
		return err
	}

	devApp := app.DevClone()
	if err := devApp.Refresh(ctx, k8sClient); err != nil {
		return err
	}
	pod, err := devApp.GetRunningPod(ctx, k8sClient)
	if err != nil {
		return err
	}

	userID := pods.GetPodUserID(ctx, pod.Name, up.Dev.Container, up.Dev.Namespace, k8sClient)
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
