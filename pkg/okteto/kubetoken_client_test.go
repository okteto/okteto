package okteto

import (
	"fmt"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestNewKubeTokenClient(t *testing.T) {
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
		t.Run(tc.testName, func(t *testing.T) {
			client, err := NewKubeTokenClient(tc.contextName, tc.token)

			require.Equal(t, tc.expectedError, err)
			require.Equal(t, tc.expectedClient, client)
		})
	}

	t.Run("Parse error", func(t *testing.T) {

		_, err := NewKubeTokenClient("not!!://a.url", "mytoken")
		require.Error(t, err)
	},
	)

	t.Run("No error", func(t *testing.T) {

		client, err := NewKubeTokenClient("cloud.okteto.com", "mytoken")
		require.NoError(t, err)

		require.Equal(t, "https://cloud.okteto.com/auth/kubetoken", client.url)
		require.Equal(t, "cloud.okteto.com", client.contextName)
	},
	)
}
