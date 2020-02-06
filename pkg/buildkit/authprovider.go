package buildkit

import (
	"context"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"google.golang.org/grpc"
)

//NewRegistryAuthProvider returns an auth provider for Okteto Cloud Registry
func NewRegistryAuthProvider(registry, username, password string) session.Attachable {
	return &authProvider{
		registry: registry,
		username: username,
		password: password,
	}
}

type authProvider struct {
	registry string
	username string
	password string
}

func (ap *authProvider) Register(server *grpc.Server) {
	auth.RegisterAuthServer(server, ap)

}

func (ap *authProvider) Credentials(ctx context.Context, req *auth.CredentialsRequest) (*auth.CredentialsResponse, error) {
	res := &auth.CredentialsResponse{}
	res.Username = ap.username
	res.Secret = ap.password
	return res, nil
}
