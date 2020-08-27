// Copyright 2020 The Okteto Authors
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

package okteto

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/machinebox/graphql"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func getClient(oktetoURL string) (*graphql.Client, error) {

	u, err := parseOktetoURL(oktetoURL)
	if err != nil {
		return nil, err
	}

	graphqlClient := graphql.NewClient(u)
	return graphqlClient, nil
}

func parseOktetoURL(u string) (string, error) {
	if u == "" {
		return "", fmt.Errorf("the okteto URL is not set")
	}

	parsed, err := url.Parse(u)
	if err != nil {
		return "", err
	}

	if parsed.Scheme == "" {
		parsed.Scheme = "https"
		parsed.Host = parsed.Path
	}

	parsed.Path = "graphql"
	return parsed.String(), nil
}

func getRequest(q, token string) *graphql.Request {
	req := graphql.NewRequest(q)
	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", token))
	return req
}

func query(ctx context.Context, query string, result interface{}) error {
	t, err := GetToken()
	if err != nil {
		log.Infof("couldn't get token: %s", err)
		return errors.ErrNotLogged
	}

	c, err := getClient(t.URL)
	if err != nil {
		log.Infof("error getting the graphql client: %s", err)
		return fmt.Errorf("internal server error")
	}

	req := getRequest(query, t.Token)
	if err := c.Run(ctx, req, result); err != nil {
		return translateAPIErr(err)
	}

	return nil
}

func translateAPIErr(err error) error {
	e := strings.TrimPrefix(err.Error(), "graphql: ")
	switch e {
	case "not-authorized":
		return errors.ErrNotLogged
	case "namespace-quota-exceeded":
		return fmt.Errorf("you have exceeded your namespace quota. Contact us at hello@okteto.com to learn more")
	case "namespace-quota-exceeded-onpremises":
		return fmt.Errorf("you have exceeded your namespace quota, please contact your administrator to increase it")
	case "users-limit-exceeded":
		return fmt.Errorf("license limit exceeded. Contact your administrator to update your license and try again")
	case "internal-server-error":
		return fmt.Errorf("server temporarily unavailable, please try again")
	default:
		log.Infof("unrecognized API error: %s", err)
		return fmt.Errorf(e)
	}

}

//SetKubeConfig updates a kubeconfig file with okteto cluster credentials
func SetKubeConfig(cred *Credential, kubeConfigPath, namespace, userName, clusterName string) error {
	contextName, namespace := getContextName(cred, namespace, clusterName)

	cfg, err := getOrCreateKubeConfig(kubeConfigPath)
	if err != nil {
		return err
	}

	// create cluster
	cluster, ok := cfg.Clusters[clusterName]
	if !ok {
		cluster = clientcmdapi.NewCluster()
	}

	cluster.CertificateAuthorityData = []byte(cred.Certificate)
	cluster.Server = cred.Server
	cfg.Clusters[clusterName] = cluster

	// create user
	user, ok := cfg.AuthInfos[userName]
	if !ok {
		user = clientcmdapi.NewAuthInfo()
	}
	user.Token = cred.Token
	cfg.AuthInfos[userName] = user

	// create context
	context, ok := cfg.Contexts[contextName]
	if !ok {
		context = clientcmdapi.NewContext()
	}

	context.Cluster = clusterName
	context.AuthInfo = userName
	context.Namespace = namespace
	cfg.Contexts[contextName] = context

	cfg.CurrentContext = contextName

	return clientcmd.WriteToFile(*cfg, kubeConfigPath)
}

//RemoveKubeConfig removes okteto cluster context from a kubeconfig file
func RemoveKubeConfig(cred *Credential, kubeConfigPath, namespace, userName, clusterName string) error {
	contextToRemoveName, _ := getContextName(cred, namespace, clusterName)
	cfg, err := getOrCreateKubeConfig(kubeConfigPath)
	if err != nil {
		return err
	}

	// check if there is context to remove
	contextToRemove, ok := cfg.Contexts[contextToRemoveName]
	if !ok {
		return nil
	}

	// check for references of cluster and user
	clusterReferenced, userReferenced := false, false
	for contextName, context := range cfg.Contexts {
		if contextName == contextToRemoveName {
			continue
		}

		if context.Cluster == contextToRemove.Cluster {
			clusterReferenced = true
		}

		if context.AuthInfo == contextToRemove.AuthInfo {
			userReferenced = true
		}
	}

	// remove cluster
	if !clusterReferenced {
		delete(cfg.Clusters, clusterName)
	}

	// remove user
	if !userReferenced {
		delete(cfg.AuthInfos, userName)
	}

	// remove context
	delete(cfg.Contexts, contextToRemoveName)

	return clientcmd.WriteToFile(*cfg, kubeConfigPath)
}

// InDevContainer returns true if running in an okteto dev container
func InDevContainer() bool {
	if v, ok := os.LookupEnv("OKTETO_NAMESPACE"); ok && v != "" {
		return true
	}

	return false
}

func getOrCreateKubeConfig(kubeConfigPath string) (*clientcmdapi.Config, error) {
	var cfg *clientcmdapi.Config
	_, err := os.Stat(kubeConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = clientcmdapi.NewConfig()
		} else {
			return nil, err
		}
	} else {
		cfg, err = clientcmd.LoadFromFile(kubeConfigPath)
		if err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func getContextName(cred *Credential, namespace, clusterName string) (string, string) {
	contextName := ""
	if namespace == "" {
		// don't include namespace for the personal namespace
		contextName = clusterName
		namespace = cred.Namespace
	} else {
		contextName = fmt.Sprintf("%s-%s", clusterName, namespace)
	}
	return contextName, namespace
}
