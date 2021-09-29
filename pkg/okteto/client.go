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

	"github.com/machinebox/graphql"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
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
	c, err := getClient(Context().Name)
	if err != nil {
		log.Infof("error getting the graphql client: %s", err)
		return fmt.Errorf("internal server error")
	}

	req := getRequest(query, Context().Token)
	if err := c.Run(ctx, req, result); err != nil {
		return translateAPIErr(err)
	}

	return nil
}

func queryWithRequest(ctx context.Context, req *graphql.Request, result interface{}) error {
	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", Context().Token))

	c, err := getClient(Context().Name)
	if err != nil {
		log.Infof("error getting the graphql client: %s", err)
		return fmt.Errorf("internal server error")
	}

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
	case "decoding response: EOF":
		return fmt.Errorf("your api token is not valid. Please run okteto login and try again")
	default:
		log.Infof("Unrecognized API error: %s", err)
		return fmt.Errorf(e)
	}

}

// InDevContainer returns true if running in an okteto dev container
func InDevContainer() bool {
	if v, ok := os.LookupEnv("OKTETO_NAME"); ok && v != "" {
		return true
	}

	return false
}
