package kubetoken

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mockStore struct {
	data     []byte
	getError error
	setError error
}

func (s *mockStore) Get() ([]byte, error) {
	return s.data, s.getError
}

func (s *mockStore) Set(value []byte) error {
	s.data = value
	return s.setError
}

func TestFileCache(t *testing.T) {

	// Test file has corrupted data

	// Test token expiration

	// Test token not expired

	now := time.Now()
	expirationTime := now.Add(time.Minute)

	token := authenticationv1.TokenRequest{
		Status: authenticationv1.TokenRequestStatus{
			ExpirationTimestamp: metav1.Time{
				Time: expirationTime,
			},
		},
	}

	context := "context"
	namespace := "namespace"

	store := []storeRegister{
		{
			ContextName: context,
			Namespace:   namespace,
			Token:       token,
		},
	}

	storeString, err := json.Marshal(store)
	require.NoError(t, err)

	c := Cache{
		StringStore: &mockStore{data: storeString},
		Now: func() time.Time {
			return now
		},
	}

	result, err := c.Get(context, namespace)
	require.NoError(t, err)

	tokenString, err := json.Marshal(token)
	require.NoError(t, err)
	require.JSONEq(t, string(tokenString), result)
}
