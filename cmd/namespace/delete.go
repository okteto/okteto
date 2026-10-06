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
	"errors"
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
)

const (
	defaultDeleteTimeout = 5 * time.Minute

	namespaceStatusActive       = "Active"
	namespaceStatusDeleteFailed = "DeleteFailed"
)

// logsGracePeriod is the maximum time to wait for the deletion logs once the wait has finished
var logsGracePeriod = 10 * time.Second

// DeleteOptions represents the options that namespace delete has
type DeleteOptions struct {
	Namespace string
	Timeout   time.Duration
	Wait      bool
}

// Delete deletes a namespace
func Delete(ctx context.Context, ioCtrl *io.Controller) *cobra.Command {
	opts := &DeleteOptions{}
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an Okteto Namespace",
		Args:  utils.MaximumNArgsAccepted(1, ""),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{}); err != nil {
				return err
			}

			opts.Namespace = okteto.GetContext().Namespace
			if len(args) > 0 {
				opts.Namespace = args[0]
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			nsCmd, err := NewCommand(ioCtrl)
			if err != nil {
				return err
			}
			err = nsCmd.ExecuteDeleteNamespace(ctx, opts)
			analytics.TrackDeleteNamespace(err == nil)
			return err
		},
	}
	cmd.Flags().BoolVarP(&opts.Wait, "wait", "w", true, "wait until the Okteto Namespace is fully deleted from the cluster")
	cmd.Flags().DurationVarP(&opts.Timeout, "timeout", "t", defaultDeleteTimeout, "the duration to wait for the Okteto Namespace to be deleted. Any value should contain a corresponding time unit e.g. 1s, 2m, 3h")
	return cmd
}

func (nc *Command) ExecuteDeleteNamespace(ctx context.Context, opts *DeleteOptions) error {
	namespace := opts.Namespace
	oktetoLog.Spinner(fmt.Sprintf("Deleting %s namespace", namespace))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	// trigger namespace deletion
	if err := nc.okClient.Namespaces().Delete(ctx, namespace); err != nil {
		if oktetoErrors.IsNotFound(err) {
			oktetoLog.Information("Namespace '%s' not found", namespace)
			return nil
		}
		return fmt.Errorf("%w: %w", errFailedDeleteNamespace, err)
	}

	if opts.Wait {
		if err := nc.watchDelete(ctx, namespace, opts.Timeout); err != nil {
			return fmt.Errorf("watching namespace deletion stopped: %w", err)
		}
		oktetoLog.Success("Namespace '%s' deleted", namespace)
	} else {
		oktetoLog.Success("Namespace '%s' scheduled for deletion", namespace)
	}

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

func (nc *Command) watchDelete(ctx context.Context, namespace string, timeout time.Duration) error {
	waitCtx, ctxCancel := context.WithCancel(ctx)
	defer ctxCancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	defer signal.Stop(stop)

	logsCtx, logsCtxCancel := context.WithCancel(waitCtx)
	defer logsCtxCancel()

	// exit is not closed because the waiting goroutine might still send to it after CTRL+C
	exit := make(chan error, 1)

	var wg sync.WaitGroup

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		exit <- nc.waitForNamespaceDeleted(waitCtx, namespace, timeout)
	}(&wg)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		connectionTimeout := 5 * time.Minute
		err := nc.okClient.Stream().DestroyAllLogs(logsCtx, namespace, connectionTimeout)
		// the logs context is canceled when the wait finishes with an error (e.g. timeout), so we should not display the warning
		if err != nil && !errors.Is(err, context.Canceled) {
			oktetoLog.Warning("delete namespace logs cannot be streamed due to connectivity issues")
			oktetoLog.Infof("delete namespace logs cannot be streamed due to connectivity issues: %v", err)
		}
	}(&wg)

	select {
	case <-stop:
		ctxCancel()
		logsCtxCancel()
		oktetoLog.Infof("CTRL+C received, exit")
		oktetoLog.Information("CTRL+C received, cancelling wait and logs streaming but operation will continue in background")
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			logsCtxCancel()
		}
		logsDone := make(chan struct{})
		go func() {
			wg.Wait()
			close(logsDone)
		}()
		// wait until streaming logs have finished, but don't block forever if the stream never ends
		select {
		case <-logsDone:
		case <-time.After(logsGracePeriod):
			oktetoLog.Infof("delete namespace logs didn't finish after %s, stop streaming", logsGracePeriod)
			logsCtxCancel()
			<-logsDone
		case <-stop:
			logsCtxCancel()
			<-logsDone
			oktetoLog.Infof("CTRL+C received, exit")
			return oktetoErrors.ErrIntSig
		}
		return err
	}
}

// waitForNamespaceDeleted polls the Okteto API until the namespace no longer exists in the cluster.
// The Okteto API is used instead of the user's kubernetes credentials because the role binding granting
// the user access to the namespace is removed while the namespace is terminating, so the user would get a
// forbidden error before the namespace is actually deleted (e.g. while waiting for finalizers)
func (nc *Command) waitForNamespaceDeleted(ctx context.Context, namespace string, timeout time.Duration) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	to := time.NewTimer(timeout)
	defer to.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-to.C:
			return fmt.Errorf("%w: namespace %s, time %s", errDeleteNamespaceTimeout, namespace, timeout.String())
		case <-ticker.C:
			ns, err := nc.okClient.Namespaces().Get(ctx, namespace)
			if err != nil {
				// not found error is expected when the namespace is deleted
				if errors.Is(err, oktetoErrors.ErrNamespaceNotFound) {
					return nil
				}
				if oktetoErrors.IsTransient(err) {
					oktetoLog.Debugf("transient error getting namespace %q status: %v", namespace, err)
					continue
				}
				return err
			}

			// If a dev environment fails to be destroyed, the okteto backend sets the namespace status back
			// to "Active" or "Sleeping" (it sets the status to "Deleting" when starting the namespace deletion)
			switch ns.Status {
			case namespaceStatusDeleteFailed, namespaceStatusActive, constants.NamespaceStatusSleeping:
				return fmt.Errorf("%w: namespace status is %q", errFailedDeleteNamespace, ns.Status)
			}
		}
	}
}
