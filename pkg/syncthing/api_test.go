package syncthing

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
