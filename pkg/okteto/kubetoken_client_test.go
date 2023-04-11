package okteto

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
		require.Equal(t, "testns", client.namespace)
	})
}

type mockCache struct {
	token    *authenticationv1.TokenRequest
	getErr   error
	setErr   error
	getCount int
	setCount int
}

func (m *mockCache) Get(_, _ string) (*authenticationv1.TokenRequest, error) {
	m.getCount++
	return m.token, m.getErr
}

func (m *mockCache) Set(_, _ string, token *authenticationv1.TokenRequest) error {
	m.setCount++
	m.token = token
	return m.setErr
}

func TestGetKubeTokenCache(t *testing.T) {
	t.Parallel()

	expectedToken := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences: []string{"test"},
		},
		Status: authenticationv1.TokenRequestStatus{
			Token: "jwt.token.test",
			ExpirationTimestamp: metav1.Time{
				Time: time.Now().Add(time.Hour),
			},
		},
	}

	expectedTokenBytes, err := json.Marshal(expectedToken)
	require.NoError(t, err)
	expectedTokenString := string(expectedTokenBytes)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(expectedTokenBytes)
	}))

	defer s.Close()

	// TODO: the tests should have better checks for the cache

	tt := []struct {
		name             string
		cache            *mockCache
		expectedGetCount int
		expectedSetCount int
	}{
		{
			name:             "Cache get error",
			cache:            &mockCache{getErr: assert.AnError},
			expectedGetCount: 1,
			expectedSetCount: 1,
		},
		{
			name:             "Cache set error",
			cache:            &mockCache{setErr: assert.AnError},
			expectedGetCount: 1,
			expectedSetCount: 1,
		},
		{
			name:             "Cache get and set error",
			cache:            &mockCache{getErr: assert.AnError, setErr: assert.AnError},
			expectedGetCount: 1,
			expectedSetCount: 1,
		},
		{
			name:             "Cache hit",
			cache:            &mockCache{token: expectedToken},
			expectedGetCount: 1,
			expectedSetCount: 0,
		},
		{
			name:             "Cache miss",
			cache:            &mockCache{token: nil},
			expectedGetCount: 1,
			expectedSetCount: 1,
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			c := &KubeTokenClient{
				httpClient: s.Client(),
				url:        s.URL,
				cache:      tc.cache,
			}

			token, err := c.GetKubeToken()
			require.NoError(t, err)
			require.Equal(t, expectedTokenString, token)

			require.Equal(t, tc.expectedGetCount, tc.cache.getCount)
			require.Equal(t, tc.expectedSetCount, tc.cache.setCount)
		})
	}

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
		cache:       &mockCache{},
	}

	_, err := c.GetKubeToken()
	require.Equal(t, fmt.Errorf(oktetoErrors.ErrNotLogged, context), err)
}
