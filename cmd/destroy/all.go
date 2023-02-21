package destroy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/okteto/okteto/cmd/utils/executor"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type localDestroyAllCommand struct {
	configMapHandler  configMapHandler
	nsDestroyer       destroyer
	executor          executor.ManifestExecutor
	oktetoClient      *okteto.OktetoClient
	secrets           secretHandler
	k8sClientProvider okteto.K8sClientProvider
}

func newLocalDestroyerAll(
	k8sClient *kubernetes.Clientset,
	k8sClientProvider okteto.K8sClientProvider,
	executor executor.ManifestExecutor,
	nsDestroyer destroyer,
	oktetoClient *okteto.OktetoClient,
) *localDestroyAllCommand {
	return &localDestroyAllCommand{
		configMapHandler:  newConfigmapHandler(k8sClient),
		secrets:           secrets.NewSecrets(k8sClient),
		k8sClientProvider: k8sClientProvider,
		oktetoClient:      oktetoClient,
		nsDestroyer:       nsDestroyer,
		executor:          executor,
	}
}

func (dc *localDestroyAllCommand) destroy(ctx context.Context, opts *Options) error {
	oktetoLog.Spinner(fmt.Sprintf("Deleting all in %s namespace", opts.Namespace))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	if err := dc.oktetoClient.Namespaces().DestroyAll(ctx, opts.Namespace, opts.DestroyVolumes); err != nil {
		return err
	}

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
		exit <- dc.waitForNamespaceDestroyAllToComplete(waitCtx, opts.Namespace)
	}(&wg)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err := dc.oktetoClient.Stream().DestroyAllLogs(waitCtx, opts.Namespace)
		if err != nil {
			oktetoLog.Warning("destroy all logs cannot be streamed due to connectivity issues")
			oktetoLog.Infof("destroy all logs cannot be streamed due to connectivity issues: %v", err)
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

func (ld *localDestroyAllCommand) waitForNamespaceDestroyAllToComplete(ctx context.Context, namespace string) error {
	timeout := 5 * time.Minute
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)

	c, _, err := ld.k8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}
	hasBeenDestroyingAll := false

	for {
		select {
		case <-to.C:
			return fmt.Errorf("'%s' deploy didn't finish after %s", namespace, timeout.String())
		case <-ticker.C:
			ns, err := c.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
			if err != nil {
				return err
			}

			status, ok := ns.Labels["space.okteto.com/status"]
			if !ok {
				// when status label is not present, continue polling the namespace until timeout
				oktetoLog.Debugf("namespace %q does not have label for status", namespace)
				continue
			}

			switch status {
			case "Active":
				if hasBeenDestroyingAll {
					// when status is active again check if all resources have been correctly destroyed
					cfgList, err := c.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
					if err != nil {
						return err
					}

					// no configmap for the given namespace should exist
					if len(cfgList.Items) > 0 {
						return fmt.Errorf("namespace destroy all failed: some resources where not destroyed")
					}
					// exit the waiting loop when status is active again
					return nil
				}
			case "DestroyingAll":
				// initial state would be active, check if this changes to assure namespace has been in destroying all status
				hasBeenDestroyingAll = true
			case "DestroyAllFailed":
				return errors.New("namespace destroy all failed")
			}
		}
	}
}
