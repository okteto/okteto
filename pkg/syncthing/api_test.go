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

package syncthing

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/vars"
	"github.com/stretchr/testify/assert"
)

type varManagerLogger struct{}

func (varManagerLogger) Yellow(_ string, _ ...interface{}) {}
func (varManagerLogger) AddMaskedWord(_ string)            {}

func TestMain(m *testing.M) {
	varManager := vars.NewVarsManager(&varManagerLogger{})
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		log.Fatal(err)
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Fatal(err)
		}
	}(tmpDir)

	varManager.AddLocalVar("HOME", tmpDir)
	vars.GlobalVarManager = varManager

	exitCode := m.Run()

	os.Exit(exitCode)
}

type testKeyTransport struct {
	RT http.RoundTripper
	t  *testing.T
}

func (t *testKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Check that a previous RoundTripper added the required headers.
	val, ok := req.Header[APIKeyHeader]
	if !ok {
		t.t.Errorf("Header %s should be set in request", APIKeyHeader)
		return t.RT.RoundTrip(req)
	}

	if val[0] != APIKeyHeaderValue {
		t.t.Errorf("Header %s should be set to %s", APIKeyHeader, APIKeyHeaderValue)
	}

	return t.RT.RoundTrip(req)
}

// Ensure that the round tripper is adding the correct headers.
func Test_RoundTrip(t *testing.T) {
	apiKeyTransport := addAPIKeyTransport{
		&testKeyTransport{
			RT: http.DefaultTransport,
			t:  t,
		},
	}

	testRequest := httptest.NewRequest(http.MethodGet, "https://foo.com", nil)
	// nolint:bodyclose
	apiKeyTransport.RoundTrip(testRequest)
}

func Test_APIClient(t *testing.T) {
	client := NewAPIClient()

	expectation := 60 * time.Second
	assert.Equal(
		t,
		expectation,
		client.Timeout,
		"timeout should be set to %s seconds",
		expectation,
	)
}
