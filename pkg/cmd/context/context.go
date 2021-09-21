// Copyright 2021 The Okteto Authors
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
	"context"
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/okteto"
)

func CopyK8sClusterConfigToOktetoContext(clusterName string) error {
	kubeConfigFile := config.GetKubeConfigFile()
	oktetoKubeconfigFile := config.GetContextKubeconfigPath()
	config, err := okteto.GetKubeConfig(kubeConfigFile)
	if err != nil {
		return err
	}

	authInfo := config.AuthInfos[clusterName]
	cluster := config.Clusters[clusterName]
	context := config.Contexts[clusterName]
	extension := config.Extensions[clusterName]
	err = okteto.SetContextFromConfigFields(oktetoKubeconfigFile, clusterName, authInfo, cluster, context, extension)
	if err != nil {
		return err
	}
	return nil
}

func SaveOktetoContext(ctx context.Context, namespace string) error {
	cred, err := okteto.GetCredentials(ctx)
	if err != nil {
		return err
	}
	if namespace == "" {
		namespace = cred.Namespace
	}

	hasAccess, err := utils.HasAccessToNamespace(ctx, namespace)
	if err != nil {
		return err
	}
	if !hasAccess {
		return fmt.Errorf(ErrNamespaceNotFound, namespace)
	}

	kubeConfigFile := config.GetContextKubeconfigPath()
	clusterContext := okteto.GetClusterContext()

	if err := okteto.SetKubeConfig(cred, kubeConfigFile, namespace, okteto.GetUserID(), clusterContext, true); err != nil {
		return err
	}

	token, err := okteto.GetToken()
	if err != nil {
		return err
	}
	return okteto.SaveContext(okteto.GetURL(), clusterContext, token.Token)
}

func SaveK8sContext(_ context.Context, clusterName, namespace string) error {
	kubeConfigFile := config.GetContextKubeconfigPath()
	config, err := okteto.GetKubeConfig(kubeConfigFile)
	if err != nil {
		return err
	}

	authInfo := config.AuthInfos[clusterName]
	cluster := config.Clusters[clusterName]
	context := config.Contexts[clusterName]
	extension := config.Extensions[clusterName]

	if namespace == "" {
		context.Namespace = namespace
	}

	err = okteto.SetContextFromConfigFields(kubeConfigFile, clusterName, authInfo, cluster, context, extension)
	if err != nil {
		return err
	}
	return okteto.SaveContext("", clusterName, "")
}

func HasBeenLogged(url string) bool {
	cc, err := okteto.GetOktetoContextConfig()
	if err != nil {
		return false
	}
	context := okteto.UrlToContext(url)
	_, ok := cc.Contexts[context]
	return ok
}

func GetApiToken(url string) string {
	cc, err := okteto.GetOktetoContextConfig()
	if err != nil {
		return ""
	}

	context := okteto.UrlToContext(url)
	return cc.Contexts[context].ApiToken
}
