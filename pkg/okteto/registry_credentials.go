package okteto

import (
	"context"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

func GetExternalRegistryCredentials(ctx context.Context, registryOrImage string) (string, string, error) {
	if !IsOkteto() {
		return "", "", nil
	}
	c, err := NewOktetoClient()
	if err != nil {
		oktetoLog.Debugf("failed to create okteto client for getting registry credentials: %s", err.Error())
		return "", "", err
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
	}

	ac, err := c.User().GetRegistryCredentials(ctx, registry)
	return ac.Username, ac.Password, err
}
