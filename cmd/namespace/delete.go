// Copyright 2022 The Okteto Authors
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
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Delete deletes a namespace
func Delete(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a namespace",
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.ContextOptions{}); err != nil {
				return err
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			nsCmd, err := NewCommand()
			if err != nil {
				return err
			}
			err = nsCmd.ExecuteDeleteNamespace(ctx, args[0])
			analytics.TrackDeleteNamespace(err == nil)
			return err
		},
		Args: utils.ExactArgsAccepted(1, ""),
	}
	return cmd
}

func (nc *NamespaceCommand) ExecuteDeleteNamespace(ctx context.Context, namespace string) error {
	oktetoLog.Spinner(fmt.Sprintf("Deleting %s namespace", namespace))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	// trigger namespace deletion
	if err := nc.okClient.Namespaces().Delete(ctx, namespace); err != nil {
		return fmt.Errorf("failed to delete namespace: %v", err)
	}

	if err := nc.watchDelete(ctx, namespace); err != nil {
		return err
	}

	oktetoLog.Success("Namespace '%s' deleted", namespace)
	if okteto.Context().Namespace == namespace {
		personalNamespace := okteto.Context().PersonalNamespace
		if personalNamespace == "" {
			personalNamespace = okteto.GetSanitizedUsername()
		}
		ctxOptions := &contextCMD.ContextOptions{
			Namespace:    personalNamespace,
			Context:      okteto.Context().Name,
			Save:         true,
			IsCtxCommand: true,
		}
		return nc.ctxCmd.Run(ctx, ctxOptions)
	}
	return nil
}

func (nc *NamespaceCommand) watchDelete(ctx context.Context, namespace string) error {
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
		exit <- nc.waitForNamespaceDeleted(waitCtx, namespace)
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

func (nc *NamespaceCommand) waitForNamespaceDeleted(ctx context.Context, namespace string) error {
	timeout := 5 * time.Minute
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)

	for {
		select {
		case <-to.C:
			return fmt.Errorf("'%s' deploy didn't finish after %s", namespace, timeout.String())
		case <-ticker.C:
			ns, err := nc.k8sClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
			if err != nil {
				// when err is NotFound return without error, as the namespace is correctly deleted
				return nil
			}

			status, ok := ns.Labels["space.okteto.com/status"]
			if !ok {
				return errors.New("namespace does not have label for status")
			}
			if status == "DeleteFailed" {
				return errors.New("namespace destroy all failed")
			}
		}
	}
}
