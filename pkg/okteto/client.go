package okteto

import (
	"context"
	"os"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/machinebox/graphql"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func getClient(oktetoURL string) (*graphql.Client, error) {
	u, err := url.Parse(oktetoURL)
	if err != nil {
		return nil, err
	}

	u.Path = path.Join(u.Path, "graphql")
	graphqlClient := graphql.NewClient(u.String())
	return graphqlClient, nil
}

func query(query string, result interface{}) error {
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
	ctx := context.Background()

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

//SetKubeConfig update a kubeconfig file with okteto cluster credentials
func SetKubeConfig(filename, namespace string) error {
	oktetoURL := GetURLWithUnderscore()
	clusterName := fmt.Sprintf("%s-cluster", oktetoURL)
	userName := GetUserID()
	contextName := fmt.Sprintf("%s-context", oktetoURL)

	var cfg *clientcmdapi.Config
	cred, err :=GetCredentials(namespace)
	if err != nil {
		return err
	}

	_, err = os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = clientcmdapi.NewConfig()
		} else {
			return err
		}
	} else {
		cfg, err = clientcmd.LoadFromFile(filename)
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
	context.Namespace = cred.Namespace
	cfg.Contexts[contextName] = context

	cfg.CurrentContext = contextName

	if err := clientcmd.WriteToFile(*cfg, filename); err != nil {
		return err
	}
	return nil
}
