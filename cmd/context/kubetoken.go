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

package context

import (
	"errors"
	"fmt"
	"sync"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	// OktetoUseStaticKubetokenEnvVar is used to opt in to use static kubetoken
	OktetoUseStaticKubetokenEnvVar = "OKTETO_USE_STATIC_KUBETOKEN"
)

var (
	usingStaticKubetokenWarningMessage = fmt.Sprintf("Using static Kubernetes token due to env var: '%s'. This feature will be removed in the future. We recommend using a dynamic kubernetes token.", OktetoUseStaticKubetokenEnvVar)
)

// staticKubetokenWarner is used to print a warning message when the static kubetoken is enabled and the cluster has dynamic kubetoken capabilities.
type staticKubetokenWarner struct {
	once sync.Once
}

// warn prints a warning message when the static kubetoken is enabled and the cluster has dynamic kubetoken capabilities.
func (wp *staticKubetokenWarner) warn() {
	wp.once.Do(func() {
		oktetoLog.Warning(usingStaticKubetokenWarningMessage)
	})
}

// staticKubetokenController is used to update the kubeconfig stored in the okteto context by using a static token to connect to the cluster.
type staticKubetokenController struct {
	staticKubetokenWarner
}

// newStaticKubetokenController creates a new staticKubetokenController
func newStaticKubetokenController() *staticKubetokenController {
	return &staticKubetokenController{
		staticKubetokenWarner: staticKubetokenWarner{},
	}
}

// updateOktetoContext when the dynamic kubetoken is disabled removes the exec from the kubeconfig stored in the okteto context
// so okteto can still use the old kubeconfig auth method (by static token) to connect to the cluster.
func (kc *staticKubetokenController) updateOktetoContextExec(okCtx *okteto.Context) error {
	kc.staticKubetokenWarner.warn()

	if okCtx == nil || okCtx.UserID == "" || okCtx.Cfg == nil || okCtx.Cfg.AuthInfos == nil || okCtx.Cfg.AuthInfos[okCtx.UserID] == nil {
		return nil
	}
	okCtx.Cfg.AuthInfos[okCtx.UserID].Exec = nil
	return nil
}

func (kc *staticKubetokenController) updateOktetoContextToken(uCtx *types.UserContext) error {
	kc.staticKubetokenWarner.warn()
	if uCtx.Credentials.Token == "" {
		return errors.New("static kubernetes token not available")
	}
	return nil
}

// dynamicKubetokenController is used to update the kubeconfig stored in the okteto context by using a dynamic token to connect to the cluster.
type dynamicKubetokenController struct {
	oktetoClientProvider oktetoClientProvider
}

// newDynamicKubetokenController creates a new dynamicKubetokenController
func newDynamicKubetokenController(okClientProvider oktetoClientProvider) *dynamicKubetokenController {
	return &dynamicKubetokenController{
		oktetoClientProvider: okClientProvider,
	}
}

// updateOktetoContext when the dynamic kubetoken is enabled configures the kubeconfig auth method to be executed by okteto kubetoken
// which returns a dynamic token to connect to the cluster.
func (dkc *dynamicKubetokenController) updateOktetoContextExec(okCtx *okteto.Context) error {
	okClient, err := dkc.oktetoClientProvider.Provide()
	if err != nil {
		return err
	}

	err = okClient.Kubetoken().CheckService(okCtx.Name, okCtx.Namespace)
	if err != nil {
		return fmt.Errorf("error checking kubetoken service: %w", err)
	}

	k8sCfg := okCtx.Cfg
	if k8sCfg.AuthInfos == nil {
		k8sCfg.AuthInfos = clientcmdapi.NewConfig().AuthInfos
		k8sCfg.AuthInfos[okCtx.UserID] = clientcmdapi.NewAuthInfo()
	}

	if token := k8sCfg.AuthInfos[okCtx.UserID].Token; token != "" {
		k8sCfg.AuthInfos[okCtx.UserID].Token = ""
	}

	k8sCfg.AuthInfos[okCtx.UserID].Exec = &clientcmdapi.ExecConfig{
		APIVersion:         "client.authentication.k8s.io/v1",
		Command:            "okteto",
		Args:               []string{"kubetoken", "--context", okCtx.Name, "--namespace", okCtx.Namespace},
		InstallHint:        "Okteto needs to be installed in your PATH and it has to be connected to your instance using the command 'okteto context use https://okteto.example.com'. Please visit https://www.okteto.com/docs/get-started/install-okteto-cli for more information.",
		InteractiveMode:    "Never",
		ProvideClusterInfo: true,
	}
	return nil
}

// updateOktetoContextToken retrieves a dynamic token for the given userContext and updates Credentials.Token
// if error while retrieving the dynamic token or flag OKTETO_USE_STATIC_KUBETOKEN is enabled, value is not updated
// and fallback to static token
func (dkc *dynamicKubetokenController) updateOktetoContextToken(userContext *types.UserContext) error {
	if userContext.User.Namespace == "" {
		return errors.New("user context namespace is empty")
	}

	c, err := dkc.oktetoClientProvider.Provide()
	if err != nil {
		return fmt.Errorf("error providing the okteto client while updating okteto context token: %w", err)
	}

	kubetoken, err := c.Kubetoken().GetKubeToken(okteto.GetContext().Name, userContext.User.Namespace)
	if err != nil || kubetoken.Status.Token == "" {
		return errors.New("dynamic kubernetes token not available: falling back to static token")
	}

	userContext.Credentials.Token = kubetoken.Status.Token
	return nil
}
