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

package okteto

import (
	"context"
	"crypto/tls"
	"os"
	"reflect"
	"testing"

	"net/http"

	"github.com/okteto/okteto/pkg/constants"
	"golang.org/x/oauth2"
)

type fakeGraphQLClient struct {
	err            error
	queryResult    interface{}
	mutationResult interface{}
}

func (fc fakeGraphQLClient) Query(ctx context.Context, q interface{}, _ map[string]interface{}) error {
	if fc.queryResult != nil {
		entityType := reflect.TypeOf(q).Elem()
		for i := 0; i < entityType.NumField(); i++ {
			value := entityType.Field(i)
			oldField := reflect.ValueOf(q).Elem().Field(i)
			newField := reflect.ValueOf(fc.queryResult).Elem().FieldByName(value.Name)
			oldField.Set(newField)
		}
	}
	return fc.err
}
func (fc fakeGraphQLClient) Mutate(ctx context.Context, m interface{}, _ map[string]interface{}) error {
	if fc.mutationResult != nil {
		if isAnInterface(m) {
			m = reflect.ValueOf(m).Elem().Interface()
		}
		entityType := reflect.TypeOf(m).Elem()
		for i := 0; i < entityType.NumField(); i++ {
			value := entityType.Field(i)
			oldField := reflect.ValueOf(m).Elem().Field(i)
			newField := reflect.ValueOf(fc.mutationResult).Elem().FieldByName(value.Name)
			oldField.Set(newField)
		}
	}
	return fc.err
}

func isAnInterface(m interface{}) bool {
	return reflect.TypeOf(m).Kind() == reflect.Ptr && reflect.TypeOf(m).Elem().Kind() == reflect.Interface
}

type fakeGraphQLMultipleCallsClient struct {
	errs           []error
	queryResults   []interface{}
	mutationResult []interface{}
}

func (fc *fakeGraphQLMultipleCallsClient) Query(ctx context.Context, q interface{}, vars map[string]interface{}) error {
	var (
		err   error
		query interface{}
	)
	if len(fc.errs) != 0 {
		err = fc.errs[0]
		newErrs := fc.errs[1:]
		fc.errs = newErrs
	}
	if len(fc.queryResults) != 0 {
		query = fc.queryResults[0]
		fc.queryResults = fc.queryResults[1:]
	}
	return fakeGraphQLClient{
		err:         err,
		queryResult: query,
	}.Query(ctx, q, vars)
}
func (fc *fakeGraphQLMultipleCallsClient) Mutate(ctx context.Context, m interface{}, vars map[string]interface{}) error {
	var (
		err      error
		mutation interface{}
	)
	if len(fc.errs) != 0 {
		err = fc.errs[0]
		fc.errs = fc.errs[1:]
	}
	if len(fc.mutationResult) != 0 {
		mutation = fc.mutationResult[0]
		fc.mutationResult = fc.mutationResult[1:]
	}
	return fakeGraphQLClient{
		err:            err,
		mutationResult: mutation,
	}.Mutate(ctx, m, vars)
}

func TestInDevContainer(t *testing.T) {
	v := os.Getenv(constants.OktetoNameEnvVar)
	t.Setenv(constants.OktetoNameEnvVar, "")
	defer func() {
		t.Setenv(constants.OktetoNameEnvVar, v)
	}()

	in := InDevContainer()
	if in {
		t.Errorf("in dev container when there was no marker env var")
	}

	t.Setenv(constants.OktetoNameEnvVar, "")
	in = InDevContainer()
	if in {
		t.Errorf("in dev container when there was an empty marker env var")
	}

	t.Setenv(constants.OktetoNameEnvVar, "1")
	in = InDevContainer()
	if !in {
		t.Errorf("not in dev container when there was a marker env var")
	}
}

func Test_parseOktetoURL(t *testing.T) {
	tests := []struct {
		name    string
		u       string
		want    string
		wantErr bool
	}{
		{
			name: "basic",
			u:    "https://cloud.okteto.com",
			want: "https://cloud.okteto.com/graphql",
		},
		{
			name: "no-schema",
			u:    "cloud.okteto.com",
			want: "https://cloud.okteto.com/graphql",
		},
		{
			name:    "empty",
			u:       "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOktetoURL(tt.u)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOktetoURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseOktetoURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBackgroundContextWithHttpClient(t *testing.T) {
	tests := []struct {
		name       string
		httpClient *http.Client
		expected   bool
	}{
		{
			name:       "default",
			httpClient: &http.Client{},
			expected:   false,
		},
		{
			name: "insecure",
			httpClient: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{ // skipcq: GO-S1020
						InsecureSkipVerify: true, // skipcq: GSC-G402
					},
				},
			},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ok bool

			ctx := contextWithOauth2HttpClient(context.Background(), tt.httpClient)

			client := ctx.Value(oauth2.HTTPClient)

			var httpClient *http.Client

			if httpClient, ok = client.(*http.Client); !ok {
				t.Errorf("got %T, want %T", client, httpClient)
				return
			}

			if !tt.expected && httpClient.Transport == nil {
				return
			}

			var transport *http.Transport

			if transport, ok = httpClient.Transport.(*http.Transport); !ok {
				t.Errorf("got %T, want %T", httpClient.Transport, transport)
				return
			}

			insecureSkipVerify := transport.TLSClientConfig.InsecureSkipVerify

			if insecureSkipVerify != tt.expected {
				t.Errorf("got %t, want %t", insecureSkipVerify, tt.expected)
			}
		})
	}
}
