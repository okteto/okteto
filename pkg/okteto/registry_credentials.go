package okteto

import (
	"context"

	dockertypes "github.com/docker/cli/cli/config/types"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
)

type RegistryCredentialsReader struct {
	OktetoClient types.UserInterface
}

func (rcr *RegistryCredentialsReader) ClusterCredentials(ctx context.Context, host string) (dockertypes.AuthConfig, error) {
	if !IsOkteto() {
		return dockertypes.AuthConfig{}, nil
	}

	c := rcr.OktetoClient

	if c == nil {
		fullClient, err := NewOktetoClient()
		if err != nil {
			oktetoLog.Debugf("failed to create okteto client for getting registry credentials: %s", err.Error())
			return dockertypes.AuthConfig{}, err
		}
		c = fullClient.User()
	}

	return c.GetRegistryCredentials(ctx, host)
}
