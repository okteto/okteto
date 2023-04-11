package okteto

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"

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

type mockCache struct {
	token    *authenticationv1.TokenRequest
	err      error
	getCount int
	setCount int
}

func (m *mockCache) Get(_, _ string) (*authenticationv1.TokenRequest, error) {
	m.getCount++
	return m.token, m.err
}

func (m *mockCache) Set(_, _ string, token *authenticationv1.TokenRequest) error {
	m.setCount++
	m.token = token
	return nil
}

func TestGetKubeTokenCache(t *testing.T) {
	t.Parallel()

	expectedToken := "token"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedToken))
	}))

	defer s.Close()

	cache := &mockCache{}

	c := &KubeTokenClient{
		httpClient: s.Client(),
		url:        s.URL,
		cache:      cache,
	}

	// TODO: the tests should have better checks for the cache
	t.Run("Cache get error", func(t *testing.T) {
		cache.token = nil
		cache.err = assert.AnError

		token, err := c.GetKubeToken()
		require.NoError(t, err)
		require.Equal(t, expectedToken, token)
		require.Equal(t, 1, cache.getCount)
		require.Equal(t, 1, cache.setCount)
	})

	t.Run("Cache set error", func(t *testing.T) {
		cache.token = nil
		cache.err = assert.AnError

		token, err := c.GetKubeToken()
		require.NoError(t, err)
		require.Equal(t, expectedToken, token)
		require.Equal(t, 1, cache.getCount)
		require.Equal(t, 1, cache.setCount)
	})

	t.Run("Cache get and set error", func(t *testing.T) {
		cache.token = nil
		cache.err = assert.AnError

		token, err := c.GetKubeToken()
		require.NoError(t, err)
		require.Equal(t, expectedToken, token)
		require.Equal(t, 1, cache.getCount)
		require.Equal(t, 1, cache.setCount)
	})

	t.Run("Cache hit", func(t *testing.T) {
		cache.token = nil
		cache.err = nil

		token, err := c.GetKubeToken()
		require.NoError(t, err)
		require.Equal(t, expectedToken, token)
		require.Equal(t, 1, cache.getCount)
		require.Equal(t, 0, cache.setCount)
	})

	t.Run("Cache miss", func(t *testing.T) {
		cache.token = nil
		cache.err = nil

		token, err := c.GetKubeToken()
		require.NoError(t, err)
		require.Equal(t, expectedToken, token)
		require.Equal(t, 1, cache.getCount)
		require.Equal(t, 1, cache.setCount)
	})

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
