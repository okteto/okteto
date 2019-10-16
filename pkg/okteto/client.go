package okteto

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/machinebox/graphql"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"

	"go.undefinedlabs.com/scopeagent/instrumentation/nethttp"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func getClient(oktetoURL string) (*graphql.Client, error) {
	u, err := url.Parse(oktetoURL)
	if err != nil {
		return nil, err
	}

	u.Path = path.Join(u.Path, "graphql")
	h := &http.Client{Transport: &nethttp.Transport{}}
	graphqlClient := graphql.NewClient(u.String(), graphql.WithHTTPClient(h))
	return graphqlClient, nil
}

func query(ctx context.Context, query string, result interface{}) error {
	o, err := getToken()
	if err != nil {
		log.Infof("couldn't get token: %s", err)
		return errors.ErrNotLogged
	}

	c, err := getClient(o.URL)
	if err != nil {
		log.Infof("error getting the graphql client: %s", err)
		return fmt.Errorf("internal server error")
	}

	req := graphql.NewRequest(query)
	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", o.Token))

	if err := c.Run(ctx, req, result); err != nil {
		e := strings.TrimPrefix(err.Error(), "graphql: ")
		if isNotAuthorized(e) {
			return errors.ErrNotLogged
		}

		if isConnectionError(e) {
			return errors.ErrInternalServerError
		}

		return fmt.Errorf(e)
	}

	return nil
}

func isNotAuthorized(s string) bool {
	return strings.Contains(s, "not-authorized")
}

func isConnectionError(s string) bool {
	return strings.Contains(s, "decoding response") || strings.Contains(s, "reading body")
}

//SetKubeConfig updates a kubeconfig file with okteto cluster credentials
func SetKubeConfig(cred *Credential, kubeConfigPath, namespace, userName, clusterName string) error {

	contextName := ""
	if len(namespace) == 0 {
		// don't include namespace for the personal namespace
		contextName = clusterName
		namespace = cred.Namespace
	} else {
		contextName = fmt.Sprintf("%s-%s", clusterName, namespace)
	}

	var cfg *clientcmdapi.Config
	_, err := os.Stat(kubeConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = clientcmdapi.NewConfig()
		} else {
			return err
		}
	} else {
		cfg, err = clientcmd.LoadFromFile(kubeConfigPath)
		if err != nil {
			return err
		}
	}

	//create cluster
	cluster, ok := cfg.Clusters[clusterName]
	if !ok {
		cluster = clientcmdapi.NewCluster()
	}

	cluster.CertificateAuthorityData = []byte(cred.Certificate)
	cluster.Server = cred.Server
	cfg.Clusters[clusterName] = cluster

	//create user
	user, ok := cfg.AuthInfos[userName]
	if !ok {
		user = clientcmdapi.NewAuthInfo()
	}
	user.Token = cred.Token
	cfg.AuthInfos[userName] = user

	//create context
	context, ok := cfg.Contexts[contextName]
	if !ok {
		context = clientcmdapi.NewContext()
	}

	context.Cluster = clusterName
	context.AuthInfo = userName
	context.Namespace = namespace
	cfg.Contexts[contextName] = context

	cfg.CurrentContext = contextName

	if err := clientcmd.WriteToFile(*cfg, kubeConfigPath); err != nil {
		return err
	}
	return nil
}
