package keda

import (
	"context"
	"fmt"

	apiextensionsclientset "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	"github.com/okteto/okteto/pkg/k8s/apps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

func UnpauseKeda(app apps.App, restConfig *rest.Config) {
	workloadName := app.ObjectMeta().Name
	namespaceName := app.ObjectMeta().Namespace
	context := context.TODO()

	apiextensionsClient, err := apiextensionsclientset.NewForConfig(restConfig)
	if err != nil {
		oktetoLog.Fail(fmt.Sprintf("Keda pause client failed for %v, err: %v", workloadName, err))
	}

	payload := `{"metadata": {"annotations": {"autoscaling.keda.sh/paused-replicas": null}}}`
	_, err = apiextensionsClient.KedaV1alpha1().ScaledObjects(namespaceName).Patch(context, workloadName, types.MergePatchType, []byte(payload), metav1.PatchOptions{})

	if err != nil {
		oktetoLog.Fail(fmt.Sprintf("Keda unpause failed for %v, err: %v", workloadName, err))
	}

	oktetoLog.Success(fmt.Sprintf("Keda unpaused %v", workloadName))
}

func PauseKeda(app apps.App, restConfig *rest.Config) {
	workloadName := app.ObjectMeta().Name
	namespaceName := app.ObjectMeta().Namespace
	context := context.TODO()

	apiextensionsClient, err := apiextensionsclientset.NewForConfig(restConfig)
	if err != nil {
		oktetoLog.Fail(fmt.Sprintf("Keda pause client failed for %v, err: %v", workloadName, err))
	}

	payload := `{"metadata": {"annotations": {"autoscaling.keda.sh/paused-replicas": "0"}}}`
	_, err = apiextensionsClient.KedaV1alpha1().ScaledObjects(namespaceName).Patch(context, workloadName, types.MergePatchType, []byte(payload), metav1.PatchOptions{})

	if err != nil {
		oktetoLog.Fail(fmt.Sprintf("Keda pause failed for %v, err: %v", workloadName, err))
	}

	oktetoLog.Success(fmt.Sprintf("Keda paused %v", workloadName))
}
