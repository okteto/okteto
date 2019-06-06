package okteto

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/machinebox/graphql"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
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
