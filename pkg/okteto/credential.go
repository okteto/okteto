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

	"github.com/shurcooL/graphql"
)

// Credential represents an Okteto Space k8s credentials
type Credential struct {
	Server      string `json:"server" yaml:"server"`
	Certificate string `json:"certificate" yaml:"certificate"`
	Token       string `json:"token" yaml:"token"`
	Namespace   string `json:"namespace" yaml:"namespace"`
}

// GetCredentials returns the space config credentials
func (c *OktetoClient) GetCredentials(ctx context.Context) (*Credential, error) {
	var query struct {
		Space struct {
			Server      graphql.String
			Certificate graphql.String
			Token       graphql.String
			Namespace   graphql.String
		} `graphql:"credentials(space: $cred)"`
	}

	variables := map[string]interface{}{
		"cred": graphql.String(""),
	}
	err := c.Query(ctx, &query, variables)
	if err != nil {
		return nil, err
	}

	cred := &Credential{
		Server:      string(query.Space.Server),
		Certificate: string(query.Space.Certificate),
		Token:       string(query.Space.Token),
		Namespace:   string(query.Space.Namespace),
	}

	if cred.Server == "" {
		return nil, fmt.Errorf("%s is not available. Please, retry again in a few minutes", Context().Name)
	}

	return cred, nil
}
