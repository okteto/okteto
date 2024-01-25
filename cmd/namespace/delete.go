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

package namespace

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Delete deletes a namespace
func Delete(ctx context.Context, k8sLogger *io.K8sLogger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a namespace",
		Args:  utils.MaximumNArgsAccepted(1, ""),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{}); err != nil {
				return err
			}

			nsToDelete := okteto.GetContext().Namespace
			if len(args) > 0 {
				nsToDelete = args[0]
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			nsCmd, err := NewCommand()
			if err != nil {
				return err
			}
			err = nsCmd.ExecuteDeleteNamespace(ctx, nsToDelete, k8sLogger)
			analytics.TrackDeleteNamespace(err == nil)
			return err
		},
	}
	return cmd
}

func (nc *Command) ExecuteDeleteNamespace(ctx context.Context, namespace string, k8sLogger *io.K8sLogger) error {
	oktetoLog.Spinner(fmt.Sprintf("Deleting %s namespace", namespace))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	// trigger namespace deletion
	if err := nc.okClient.Namespaces().Delete(ctx, namespace); err != nil {
		return fmt.Errorf("%w: %w", errFailedDeleteNamespace, err)
	}

	if err := nc.watchDelete(ctx, namespace, k8sLogger); err != nil {
		return err
	}

	oktetoLog.Success("Namespace '%s' deleted", namespace)
	if okteto.GetContext().Namespace == namespace {
		personalNamespace := okteto.GetContext().PersonalNamespace
		if personalNamespace == "" {
			personalNamespace = okteto.GetSanitizedUsername()
		}
		ctxOptions := &contextCMD.Options{
			Namespace:    personalNamespace,
			Context:      okteto.GetContext().Name,
			Save:         true,
			IsCtxCommand: true,
		}
		return nc.ctxCmd.Run(ctx, ctxOptions)
	}
	return nil
}

func (nc *Command) watchDelete(ctx context.Context, namespace string, k8sLogger *io.K8sLogger) error {
	waitCtx, ctxCancel := context.WithCancel(ctx)
	defer ctxCancel()

	stop := make(chan os.Signal, 1)
	defer close(stop)

	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)
	defer close(exit)

	var wg sync.WaitGroup

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		exit <- nc.waitForNamespaceDeleted(waitCtx, namespace, k8sLogger)
	}(&wg)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err := nc.okClient.Stream().DestroyAllLogs(waitCtx, namespace)
		if err != nil {
			oktetoLog.Warning("delete namespace logs cannot be streamed due to connectivity issues")
			oktetoLog.Infof("delete namespace logs cannot be streamed due to connectivity issues: %v", err)
		}
	}(&wg)

	select {
	case <-stop:
		ctxCancel()
		oktetoLog.Infof("CTRL+C received, exit")
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		// wait until streaming logs have finished
		wg.Wait()
		return err
	}
}

func (nc *Command) waitForNamespaceDeleted(ctx context.Context, namespace string, k8sLogger *io.K8sLogger) error {
	timeout := 5 * time.Minute
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)

	// provide k8s
	c, _, err := nc.k8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, k8sLogger)
	if err != nil {
		return err
	}

	for {
		select {
		case <-to.C:
			return fmt.Errorf("%w: namespace %s, time %s", errDeleteNamespaceTimeout, namespace, timeout.String())
		case <-ticker.C:
			ns, err := c.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
			if err != nil {
				// not found error is expected when the namespace is deleted.
				// adding also IsForbidden as in some versions kubernetes returns this status when the namespace is deleted
				// one or the other we assume the namespace has been deleted
				if k8sErrors.IsNotFound(err) || k8sErrors.IsForbidden(err) {
					return nil
				}
				return err
			}

			status, ok := ns.Labels[constants.NamespaceStatusLabel]
			if !ok {
				// when status label is not present, continue polling the namespace until timeout
				oktetoLog.Debugf("namespace %q does not have label for status", namespace)
				continue
			}
			if status == "DeleteFailed" {
				return errFailedDeleteNamespace
			}
		}
	}
}
