package okteto

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	dockertypes "github.com/docker/cli/cli/config/types"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

type externalRegistryCredentialsReader struct {
	getter   func(ctx context.Context, host string) (dockertypes.AuthConfig, error)
	isOkteto bool
}

func (r *externalRegistryCredentialsReader) read(ctx context.Context, registryOrImage string) (string, string, error) {
	if !r.isOkteto {
		return "", "", nil
	}

	registry := registryOrImage
	registry = strings.TrimPrefix(registry, "https://")
	registry = strings.TrimPrefix(registry, "http://")

	switch {
	case strings.HasPrefix(registry, "index.docker.io/v2"):
		registry = "https://index.docker.io/v2/"
	case strings.HasPrefix(registry, "index.docker.io/v1"):
		registry = "https://index.docker.io/v1/"
	case strings.HasPrefix(registry, "index.docker.io"):
		registry = "https://index.docker.io/v1/"
	default:
		u, err := url.Parse(fmt.Sprintf("//%s", registry))
		if err != nil {
			oktetoLog.Debugf("invalid registry host: %s", err.Error())
			return "", "", err
		}
		registry = u.Host
	}

	ac, err := r.getter(ctx, registry)
	return ac.Username, ac.Password, err
}

func GetExternalRegistryCredentials(ctx context.Context, registryOrImage string) (string, string, error) {
	c, err := NewOktetoClient()
	if err != nil {
		oktetoLog.Debugf("failed to create okteto client for getting registry credentials: %s", err.Error())
		return "", "", err
	}
	r := &externalRegistryCredentialsReader{
		isOkteto: IsOkteto(),
		getter:   c.User().GetRegistryCredentials,
	}
	return r.read(ctx, registryOrImage)
}
