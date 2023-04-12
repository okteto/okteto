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

	expirationTime := time.Now()

	token := authenticationv1.TokenRequest{
		Status: authenticationv1.TokenRequestStatus{
			ExpirationTimestamp: metav1.Time{
				Time: expirationTime,
			},
		},
	}
	tokenString, err := json.Marshal(token)
	require.NoError(t, err)

	context := "context"
	namespace := "namespace"

	store := []storeRegister{
		{
			ContextName: context,
			Namespace:   namespace,
			Token:       token,
		},
		{
			ContextName: "other-context",
			Namespace:   "other-namespace",
		},
	}

	storeString, err := json.Marshal(store)
	require.NoError(t, err)

	tt := []struct {
		name      string
		context   string
		ns        string
		storeData []byte
		want      string
		now       time.Time
	}{
		{
			name:      "cache hit",
			want:      string(tokenString),
			context:   context,
			ns:        namespace,
			storeData: storeString,
			now:       expirationTime.Add(-time.Minute),
		},
		{
			name:      "cache miss",
			want:      "",
			context:   "other-context",
			ns:        "other-namespace",
			storeData: storeString,
		},
		{
			name:    "empty cache",
			want:    "",
			context: context,
			ns:      namespace,
		},
		{
			name:      "expired",
			want:      "",
			context:   context,
			ns:        namespace,
			storeData: storeString,
			now:       expirationTime,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c := Cache{
				StringStore: &mockStore{data: tc.storeData},
				Now: func() time.Time {
					return tc.now
				},
			}

			result, err := c.Get(tc.context, tc.ns)
			require.NoError(t, err)

			if len(tc.want) == 0 {
				require.Empty(t, result)
			} else {
				require.JSONEq(t, string(tc.want), result)
			}
		})
	}

}
