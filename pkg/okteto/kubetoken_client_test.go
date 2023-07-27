package okteto

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewKubeTokenClient(t *testing.T) {
	t.Parallel()

	tt := []struct {
		testName       string
		contextName    string
		token          string
		expectedError  error
		expectedClient *KubeTokenClient
	}{
		{

			testName:       "No token",
			contextName:    "test",
			token:          "",
			expectedError:  fmt.Errorf(oktetoErrors.ErrNotLogged, "test"),
			expectedClient: nil,
		},
		{
			testName:       "No context",
			contextName:    "",
			token:          "test",
			expectedError:  oktetoErrors.ErrCtxNotSet,
			expectedClient: nil,
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			client, err := NewKubeTokenClient(tc.contextName, tc.token, "test")

			require.Equal(t, tc.expectedError, err)
			require.Equal(t, tc.expectedClient, client)
		})
	}

	t.Run("Parse error", func(t *testing.T) {
		t.Parallel()

		_, err := NewKubeTokenClient("not!!://a.url", "mytoken", "test")
		require.Error(t, err)
	})

	t.Run("No error", func(t *testing.T) {
		t.Parallel()

		client, err := NewKubeTokenClient("cloud.okteto.com", "mytoken", "testns")
		require.NoError(t, err)

		require.Equal(t, "https://cloud.okteto.com/auth/kubetoken/testns", client.url)
		require.Equal(t, "cloud.okteto.com", client.contextName)
	})
}

func TestGetKubeToken(t *testing.T) {
	t.Parallel()

	expectedToken := "token"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(expectedToken))
		assert.NoError(t, err)
	}))

	defer s.Close()

	c := &KubeTokenClient{
		httpClient: s.Client(),
		url:        s.URL,
	}

	token, err := c.GetKubeToken()
	require.NoError(t, err)
	require.Equal(t, expectedToken, token)
}

func TestGetKubeTokenUnauthorizedErr(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	defer s.Close()

	context := "testctx"

	c := &KubeTokenClient{
		httpClient:  s.Client(),
		url:         s.URL,
		contextName: context,
	}

	_, err := c.GetKubeToken()
	require.Equal(t, fmt.Errorf(oktetoErrors.ErrNotLogged, context), err)
}
