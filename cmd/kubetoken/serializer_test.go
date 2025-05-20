package kubetoken

import (
	"encoding/json"
	"testing"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
)

func TestSerializerAddsKind(t *testing.T) {
	serializer := &Serializer{}
	resp := types.KubeTokenResponse{
		TokenRequest: authenticationv1.TokenRequest{
			Status: authenticationv1.TokenRequestStatus{
				Token: "token",
			},
		},
	}

	out, err := serializer.ToJson(resp)
	assert.NoError(t, err)

	var data map[string]interface{}
	err = json.Unmarshal([]byte(out), &data)
	assert.NoError(t, err)
	assert.Equal(t, "ExecCredential", data["kind"])
}
