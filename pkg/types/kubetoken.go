package types

import (
	authenticationv1 "k8s.io/api/authentication/v1"
)

type KubeTokenResponse struct {
	authenticationv1.TokenRequest
}
