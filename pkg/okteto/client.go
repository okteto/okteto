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

package okteto

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/shurcooL/graphql"
	"golang.org/x/oauth2"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

//Client implementation to connect to Okteto API
type OktetoClient struct {
	client *graphql.Client
}

//NewClient creates a new client to connect with Okteto API
func NewOktetoClient() (*OktetoClient, error) {
	t, err := GetToken()
	if err != nil {
		log.Infof("couldn't get token: %s", err)
		return nil, errors.ErrNotLogged
	}
	u, err := parseOktetoURL(t.URL)
	if err != nil {
		return nil, err
	}

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: t.Token,
			TokenType: "Bearer"},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	client := &OktetoClient{
		client: graphql.NewClient(u, httpClient),
	}
	return client, nil
}

//NewClient creates a new client to connect with Okteto API
func NewOktetoClientFromUrlAndToken(url, token string) (*OktetoClient, error) {
	u, err := parseOktetoURL(url)
	if err != nil {
		return nil, err
	}

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token,
			TokenType: "Bearer"},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	client := &OktetoClient{
		client: graphql.NewClient(u, httpClient),
	}
	return client, nil
}

//NewClient creates a new client to connect with Okteto API
func NewOktetoClientFromUrl(url string) (*OktetoClient, error) {
	u, err := parseOktetoURL(url)
	if err != nil {
		return nil, err
	}

	httpClient := oauth2.NewClient(context.Background(), nil)
	client := &OktetoClient{
		client: graphql.NewClient(u, httpClient),
	}
	return client, nil
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
	case "non-200 OK status code: 401 Unauthorized body: \"\"":
		return fmt.Errorf("unauthorized. Please run okteto login and try again")
	default:
		log.Infof("Unrecognized API error: %s", err)
		return fmt.Errorf(e)
	}

}

//SetKubeConfig updates a kubeconfig file with okteto cluster credentials
func SetKubeConfig(cred *Credential, kubeConfigPath, namespace, userName, clusterName string, setCurrent bool) error {
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
	context, ok := cfg.Contexts[clusterName]
	if !ok {
		context = clientcmdapi.NewContext()
	}

	context.Cluster = clusterName
	context.AuthInfo = userName
	context.Namespace = namespace
	cfg.Contexts[clusterName] = context

	if setCurrent {
		cfg.CurrentContext = clusterName
	}

	return clientcmd.WriteToFile(*cfg, kubeConfigPath)
}

// InDevContainer returns true if running in an okteto dev container
func InDevContainer() bool {
	if v, ok := os.LookupEnv("OKTETO_NAME"); ok && v != "" {
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
