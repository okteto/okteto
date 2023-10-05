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

package exec

import (
	"context"
	"fmt"
	"io"
	"strings"

	dockerterm "github.com/moby/term"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	kexec "k8s.io/kubectl/pkg/cmd/exec"
)

// Exec executes the command in the development container
func Exec(ctx context.Context, c kubernetes.Interface, config *rest.Config, podNamespace, podName, container string, tty bool, stdin io.Reader, stdout, stderr io.Writer, command []string) error {
	// dockerterm.StdStreams() configures the terminal on windows
	dockerterm.StdStreams()

	p := &kexec.ExecOptions{}

	p.Config = config
	p.Command = command
	p.Executor = &kexec.DefaultRemoteExecutor{}
	p.IOStreams = genericclioptions.IOStreams{In: stdin, Out: stdout, ErrOut: stderr}
	p.Stdin = true
	p.TTY = tty

	t := p.SetupTTY()

	var sizeQueue remotecommand.TerminalSizeQueue
	if t.Raw {
		// this call spawns a goroutine to monitor/update the terminal size
		sizeQueue = t.MonitorSize(t.GetSize())

		// unset p.Err if it was previously set because both stdout and stderr go over p.Out when tty is
		// true
		p.ErrOut = nil
	}

	fn := func() error {
		req := c.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(podName).
			Namespace(podNamespace).
			SubResource("exec").
			Param("container", container)
		req.VersionedParams(&apiv1.PodExecOptions{
			Container: container,
			Command:   p.Command,
			Stdin:     p.Stdin,
			Stdout:    p.Out != nil,
			Stderr:    p.ErrOut != nil,
			TTY:       t.Raw,
		}, scheme.ParameterCodec)

		done := make(chan error, 1)
		go func() {
			done <- p.Executor.Execute("POST", req.URL(), config, p.In, p.Out, p.ErrOut, t.Raw, sizeQueue)
		}()

		select {
		case e := <-done:
			return e
		case <-ctx.Done():
			return nil
		}
	}

	if err := t.Safe(fn); err != nil {
		if strings.Contains(err.Error(), "exit code 130") {
			return nil
		}
		if strings.Contains(err.Error(), "exit code 137") {
			return fmt.Errorf("Connection lost to your development container. Check the logs for more information.")
		}

		return err
	}

	return nil
}
