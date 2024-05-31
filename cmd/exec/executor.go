// Copyright 2024 The Okteto Authors
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

package exec

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/okteto/cmd/up"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/exec"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/ssh"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	// defaultStdin is the default stdin
	defaultStdin = os.Stdin

	// defaultStdout is the default stdout
	defaultStdout = os.Stdout

	// defaultStderr is the default stderr
	defaultStderr = os.Stderr
)

type executor interface {
	execute(ctx context.Context, cmd []string) error
}

type executorProvider struct {
	ioCtrl            *io.Controller
	k8sClientProvider okteto.K8sClientProvider
}

func (e executorProvider) provide(dev *model.Dev, podName string) (executor, error) {
	k8sClient, cfg, err := e.k8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return nil, err
	}
	if dev.IsHybridModeEnabled() {
		e.ioCtrl.Logger().Info("Using hybrid executor")
		return &hybridExecutor{
			dev:       dev,
			k8sClient: k8sClient,
		}, nil
	}
	if dev.RemoteModeEnabled() {
		e.ioCtrl.Logger().Info("Using remote executor")
		return &sshExecutor{
			dev: dev,
		}, nil
	}
	return &k8sExecutor{
		k8sClient: k8sClient,
		cfg:       cfg,
		namespace: dev.Namespace,
		podName:   podName,
		container: dev.Container,
	}, nil
}

type hybridExecutor struct {
	dev       *model.Dev
	k8sClient kubernetes.Interface
}

func (h *hybridExecutor) execute(ctx context.Context, cmdToExec []string) error {
	hybridCtx := &up.HybridExecCtx{
		Dev:       h.dev,
		Workdir:   h.dev.Workdir,
		Name:      h.dev.Name,
		Namespace: h.dev.Namespace,
		Client:    h.k8sClient,
	}
	executor, err := up.NewHybridExecutor(ctx, hybridCtx)
	if err != nil {
		return err
	}

	cmd, err := executor.GetCommandToExec(cmdToExec)
	if err != nil {
		return err
	}

	return executor.RunCommand(cmd)
}

type sshExecutor struct {
	dev *model.Dev
}

func (s *sshExecutor) execute(ctx context.Context, cmd []string) error {
	devName := s.dev.Name
	if s.dev.Autocreate {
		devName = strings.TrimSuffix(s.dev.Name, "-okteto")
	}
	p, err := ssh.GetPort(devName)
	if err != nil {
		return oktetoErrors.UserError{
			E:    fmt.Errorf("development mode is not enabled on your deployment"),
			Hint: "Run 'okteto up' to enable it and try again",
		}
	}
	s.dev.LoadRemote(ssh.GetPublicKey())
	return ssh.Exec(
		ctx,
		s.dev.Interface,
		p,
		true,
		defaultStdin,
		defaultStdout,
		defaultStderr,
		cmd)
}

type k8sExecutor struct {
	k8sClient kubernetes.Interface
	cfg       *rest.Config
	namespace string
	podName   string
	container string
}

func (k *k8sExecutor) execute(ctx context.Context, cmd []string) error {
	return exec.Exec(
		ctx,
		k.k8sClient,
		k.cfg,
		k.namespace,
		k.podName,
		k.container,
		true,
		defaultStdin,
		defaultStdout,
		defaultStderr,
		cmd)
}
