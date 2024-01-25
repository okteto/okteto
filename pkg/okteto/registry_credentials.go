// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package okteto

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	dockertypes "github.com/docker/cli/cli/config/types"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

var globalRegistryCredentialsCache registryCache

type registryCacheItem struct {
	user string
	pass string
}

type registryCache struct {
	cache map[string]registryCacheItem
	m     sync.RWMutex
}

func (rc *registryCache) Get(host string) (user string, pass string, ok bool) {
	rc.m.RLock()
	defer rc.m.RUnlock()

	var item registryCacheItem
	item, ok = rc.cache[host]

	if ok {
		user = item.user
		pass = item.pass
	}
	return
}

func (rc *registryCache) Set(host, user, pass string) {
	rc.m.Lock()
	defer rc.m.Unlock()

	if rc.cache == nil {
		rc.cache = make(map[string]registryCacheItem)
	}

	rc.cache[host] = registryCacheItem{user, pass}
}

type externalRegistryCredentialsReader struct {
	getter   func(ctx context.Context, host string) (dockertypes.AuthConfig, error)
	cache    *registryCache
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

	if r.cache != nil {
		if user, pass, ok := r.cache.Get(registry); ok {
			return user, pass, nil
		}

		ac, err := r.getter(ctx, registry)
		if err != nil {
			return "", "", err
		}
		r.cache.Set(registry, ac.Username, ac.Password)
		return ac.Username, ac.Password, err
	}

	ac, err := r.getter(ctx, registry)
	return ac.Username, ac.Password, err
}

func GetExternalRegistryCredentialsWithContextStateless(ctx context.Context, registryOrImage string, isOkteto bool, c *Client) (string, string, error) {
	r := &externalRegistryCredentialsReader{
		isOkteto: isOkteto,
		getter:   c.User().GetRegistryCredentials,
		cache:    &globalRegistryCredentialsCache,
	}
	oktetoLog.Infof("Obtaining credentials for %q ...", registryOrImage)
	return r.read(ctx, registryOrImage)
}

// GetExternalRegistryCredentialsStateless returns registry credentials for a registry
// defined in okteto.
// This function is mostly executed by internal libraries (registry, docker
// credentials helpers, etc) and we need to respect this signature.
// For this reason, context is managed internally by the function.
func GetExternalRegistryCredentialsStateless(registryOrImage string, isOkteto bool, c *Client) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return GetExternalRegistryCredentialsWithContextStateless(ctx, registryOrImage, isOkteto, c)
}

func GetExternalRegistryCredentialsWithContext(ctx context.Context, registryOrImage string) (string, string, error) {
	c, err := NewOktetoClient()
	if err != nil {
		oktetoLog.Debugf("failed to create okteto client for getting registry credentials: %s", err.Error())
		return "", "", err
	}
	r := &externalRegistryCredentialsReader{
		isOkteto: IsOkteto(),
		getter:   c.User().GetRegistryCredentials,
		cache:    &globalRegistryCredentialsCache,
	}
	oktetoLog.Infof("Obtaining credentials for %q ...", registryOrImage)
	return r.read(ctx, registryOrImage)
}

// GetExternalRegistryCredentials returns registry credentials for a registry
// defined in okteto without rely on an okteto context global var.
// This function is mostly executed by internal libraries (registry, docker
// credentials helpers, etc) and we need to respect this signature.
// For this reason, context is managed internally by the function.
func GetExternalRegistryCredentials(registryOrImage string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return GetExternalRegistryCredentialsWithContext(ctx, registryOrImage)
}
