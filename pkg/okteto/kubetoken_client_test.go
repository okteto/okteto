package okteto

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetKubeToken(t *testing.T) {
	tests := []struct {
		name            string
		httpFakeHandler http.Handler
		namespace       string
		expectedToken   string
		expectedErr     error
	}{
		{
			name: "error request status unauthorized",
			httpFakeHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			}),
			expectedErr: errUnauthorized,
		},
		{
			name: "error request not success",
			httpFakeHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}),
			expectedErr: errStatus,
		},
		{
			name: "success response",
			httpFakeHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("token"))
			}),
			expectedToken: "token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeHttpServer := httptest.NewServer(tt.httpFakeHandler)
			defer fakeHttpServer.Close()

			fakeKubetokenClient := &kubeTokenClient{
				httpClient: fakeHttpServer.Client(),
			}

			got, err := fakeKubetokenClient.GetKubeToken(fakeHttpServer.URL, tt.namespace)
			assert.Equal(t, tt.expectedToken, got)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
