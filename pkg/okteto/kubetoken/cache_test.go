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

func TestFileCacheGet(t *testing.T) {
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

	store := storeRegistry{
		key{context, namespace}: {
			Token: token,
		},
		key{"oth", "er"}: {},
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

func TestFileCacheSet(t *testing.T) {
	stringStore := &mockStore{
		data: []byte("[{\" corrupted data"),
	}
	c := Cache{
		StringStore: stringStore,
	}

	token := authenticationv1.TokenRequest{}

	context := "context"
	namespace := "namespace"
	expectedStore := storeRegistry{
		key{context, namespace}: {
			Token: token,
		},
	}

	expectedStoreString, err := json.Marshal(expectedStore)
	require.NoError(t, err)

	err = c.setWithErr("context", "namespace", token)
	require.NoError(t, err)

	require.JSONEq(t, string(expectedStoreString), string(stringStore.data))

}

func TestUpdateStore(t *testing.T) {
	t1 := authenticationv1.TokenRequest{
		Status: authenticationv1.TokenRequestStatus{
			ExpirationTimestamp: metav1.Time{
				Time: time.Now(),
			},
		},
	}
	t2 := authenticationv1.TokenRequest{
		Status: authenticationv1.TokenRequestStatus{
			ExpirationTimestamp: metav1.Time{
				Time: time.Now().Add(time.Hour),
			},
		},
	}
	t3 := authenticationv1.TokenRequest{
		Status: authenticationv1.TokenRequestStatus{
			ExpirationTimestamp: metav1.Time{
				Time: time.Now().Add(time.Hour * 2),
			},
		},
	}
	t4 := authenticationv1.TokenRequest{
		Status: authenticationv1.TokenRequestStatus{
			ExpirationTimestamp: metav1.Time{
				Time: time.Now().Add(time.Hour * 3),
			},
		},
	}

	tt := []struct {
		name      string
		context   string
		ns        string
		token     authenticationv1.TokenRequest
		store     storeRegistry
		wantStore storeRegistry
	}{
		{
			name:    "new entry",
			context: "c4",
			ns:      "n4",
			token:   t4,
			store: storeRegistry{
				key{"c1", "n1"}: {
					Token: t1,
				},
				key{"c2", "n2"}: {
					Token: t2,
				},
				key{"c3", "n3"}: {
					Token: t3,
				},
			},
			wantStore: storeRegistry{
				key{"c1", "n1"}: {
					Token: t1,
				},
				key{"c2", "n2"}: {
					Token: t2,
				},
				key{"c3", "n3"}: {
					Token: t3,
				},
				key{"c4", "n4"}: {
					Token: t4,
				}},
		},
		{
			name:    "update entry",
			context: "c2",
			ns:      "n2",
			token:   t4,
			store: storeRegistry{
				key{"c1", "n1"}: {
					Token: t1,
				},
				key{"c2", "n2"}: {
					Token: t2,
				},
				key{"c3", "n3"}: {
					Token: t3,
				},
			},
			wantStore: storeRegistry{
				key{"c1", "n1"}: {
					Token: t1,
				},
				key{"c2", "n2"}: {
					Token: t4,
				},
				key{"c3", "n3"}: {
					Token: t3,
				},
			},
		},
		{
			name:    "empty store",
			context: "c1",
			ns:      "n1",
			token:   t1,
			store:   storeRegistry{},
			wantStore: storeRegistry{
				key{"c1", "n1"}: {
					Token: t1,
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			updateStore(tc.store, tc.context, tc.ns, tc.token)
			require.EqualValues(t, tc.wantStore, tc.store)
		})
	}
}
