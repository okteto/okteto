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

			client, err := NewKubeTokenClient(tc.contextName, tc.token, "test", &mockCache{})

			require.Equal(t, tc.expectedError, err)
			require.Equal(t, tc.expectedClient, client)
			require.Equal(t, tc.contextName, client.contextName)
		})
	}

	t.Run("Parse error", func(t *testing.T) {
		t.Parallel()

		_, err := NewKubeTokenClient("not!!://a.url", "mytoken", "test", &mockCache{})
		require.Error(t, err)
	})

	t.Run("No error", func(t *testing.T) {
		t.Parallel()

		client, err := NewKubeTokenClient("cloud.okteto.com", "mytoken", "testns", &mockCache{})
		require.NoError(t, err)

		require.Equal(t, "https://cloud.okteto.com/auth/kubetoken/testns", client.url)
		require.Equal(t, "cloud.okteto.com", client.contextName)
		require.Equal(t, "testns", client.namespace)
	})
}

type mockCache struct {
	token authenticationv1.TokenRequest
}

func (m *mockCache) Set(_, _ string, token authenticationv1.TokenRequest) {
	m.token = token
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

	tt := []struct {
		name               string
		cache              *mockCache
		expectedCacheToken string
	}{
		{
			name:               "Cache set error",
			cache:              &mockCache{},
			expectedCacheToken: "",
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
