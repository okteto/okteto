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

package preview

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
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type destroyPreviewCommand struct {
	okClient  types.OktetoInterface
	k8sClient kubernetes.Interface
}

func newDestroyPreviewCommand(okClient types.OktetoInterface, k8sClient kubernetes.Interface) destroyPreviewCommand {
	return destroyPreviewCommand{
		okClient,
		k8sClient,
	}
}

type DestroyOptions struct {
	name string
	wait bool
}

// Destroy destroy a preview
func Destroy(ctx context.Context) *cobra.Command {
	opts := &DestroyOptions{}
	cmd := &cobra.Command{
		Use:   "destroy <name>",
		Short: "Destroy a preview environment",
		Args:  utils.ExactArgsAccepted(1, ""),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.name = getExpandedName(args[0])

			ctxResource := &model.ContextResource{}
			if err := ctxResource.UpdateNamespace(opts.name); err != nil {
				return err
			}

			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{}); err != nil {
				return err
			}
			oktetoLog.Information("Using %s @ %s as context", opts.name, okteto.RemoveSchema(okteto.GetContext().Name))

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			oktetoClient, err := okteto.NewOktetoClient()
			if err != nil {
				return err
			}
			k8sClient, _, err := okteto.GetK8sClient()
			if err != nil {
				return err
			}
			c := newDestroyPreviewCommand(oktetoClient, k8sClient)

			err = c.executeDestroyPreview(ctx, opts)
			analytics.TrackPreviewDestroy(err == nil)
			return err
		},
	}
	cmd.Flags().BoolVarP(&opts.wait, "wait", "w", true, "wait until the preview environment gets destroyed (defaults to true)")
	return cmd
}

func (c destroyPreviewCommand) executeDestroyPreview(ctx context.Context, opts *DestroyOptions) error {
	oktetoLog.Spinner(fmt.Sprintf("Destroying %q preview environment", opts.name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	if err := c.okClient.Previews().Destroy(ctx, opts.name); err != nil {
		return fmt.Errorf("failed to destroy preview environment: %w", err)
	}

	if !opts.wait {
		oktetoLog.Success("Preview environment '%s' scheduled to destroy", opts.name)
		return nil
	}

	if err := c.watchDestroy(ctx, opts.name); err != nil {
		return err
	}

	oktetoLog.Success("Preview environment '%s' successfully destroyed", opts.name)
	return nil
}

func (c destroyPreviewCommand) watchDestroy(ctx context.Context, preview string) error {
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
		exit <- c.waitForPreviewDestroyed(waitCtx, preview)
	}(&wg)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err := c.okClient.Stream().DestroyAllLogs(waitCtx, preview)
		if err != nil {
			oktetoLog.Warning("final log output will appear once this action is complete")
			oktetoLog.Infof("destroy preview logs cannot be streamed due to connectivity issues: %v", err)
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

func (c destroyPreviewCommand) waitForPreviewDestroyed(ctx context.Context, preview string) error {
	timeout := 5 * time.Minute
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)

	for {
		select {
		case <-to.C:
			return fmt.Errorf("%w: preview %s, time %s", errDestroyPreviewTimeout, preview, timeout.String())
		case <-ticker.C:
			ns, err := c.k8sClient.CoreV1().Namespaces().Get(ctx, preview, metav1.GetOptions{})
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
				oktetoLog.Debugf("namespace %q does not have label for status", preview)
				continue
			}
			// We should also check "Active" status. If a dev environment fails to be destroyed, we go back the preview status
			// to "Active" (okteto backend sets the status to deleting when starting the preview deletion)
			if status == "DeleteFailed" || status == "Active" {
				return errFailedDestroyPreview
			}
		}
	}
}
