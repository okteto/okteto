// Copyright 2022 The Okteto Authors
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
	"os"
	"testing"

	"net/http"

	"github.com/okteto/okteto/pkg/model"
	"golang.org/x/oauth2"
)

func TestInDevContainer(t *testing.T) {
	v := os.Getenv(model.OktetoNameEnvVar)
	os.Setenv(model.OktetoNameEnvVar, "")
	defer func() {
		os.Setenv(model.OktetoNameEnvVar, v)
	}()

	in := InDevContainer()
	if in {
		t.Errorf("in dev container when there was no marker env var")
	}

	os.Setenv(model.OktetoNameEnvVar, "")
	in = InDevContainer()
	if in {
		t.Errorf("in dev container when there was an empty marker env var")
	}

	os.Setenv(model.OktetoNameEnvVar, "1")
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
		name     string
		value    bool
		expected bool
	}{
		{
			name:     "default",
			value:    false,
			expected: false,
		},
		{
			name:     "insecure",
			value:    true,
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ok bool

			ctx := backgroundContextWithHttpClient(tt.value)

			client := ctx.Value(oauth2.HTTPClient)

			var httpClient *http.Client

			if httpClient, ok = client.(*http.Client); !ok {
				t.Errorf("got %T, want %T", client, httpClient)
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
