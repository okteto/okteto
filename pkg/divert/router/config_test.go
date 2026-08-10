// Copyright 2026 The Okteto Authors
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

package router

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeEnv turns a map into the getenv function LoadConfig expects, so the loader can be
// tested without mutating process state.
func fakeEnv(vars map[string]string) func(string) string {
	return func(name string) string {
		return vars[name]
	}
}

// validEnv is a complete, minimal environment. Tests copy it and change one thing.
func validEnv() map[string]string {
	return map[string]string{
		EnvServiceName:     "api",
		EnvSharedNamespace: "staging",
		EnvBaselineHost:    "api-baseline.staging.svc.cluster.local",
		EnvPorts:           `[{"name":"http","listen":8080,"service":80}]`,
	}
}

func envWithout(name string) map[string]string {
	vars := validEnv()
	delete(vars, name)
	return vars
}

func envWith(name, value string) map[string]string {
	vars := validEnv()
	vars[name] = value
	return vars
}

func TestLoadConfig_Defaults(t *testing.T) {
	cfg, err := LoadConfig(fakeEnv(validEnv()))

	require.NoError(t, err)
	require.Equal(t, "api", cfg.Service)
	require.Equal(t, "staging", cfg.SharedNamespace)
	require.Equal(t, "api-baseline.staging.svc.cluster.local", cfg.BaselineHost)
	require.Equal(t, defaultHealthPort, cfg.HealthPort)
	require.Equal(t, defaultMaxHops, cfg.MaxHops)
	require.Empty(t, cfg.Routes)
	require.Empty(t, cfg.RoutesDir)
}

func TestLoadConfig_SinglePort(t *testing.T) {
	cfg, err := LoadConfig(fakeEnv(validEnv()))

	require.NoError(t, err)
	require.Equal(t, []PortConfig{{Name: "http", Listen: 8080, Service: 80}}, cfg.Ports)
}

// A Service exposing two ports is unusable if a divert only carries one of them.
func TestLoadConfig_SeveralPorts(t *testing.T) {
	vars := envWith(EnvPorts, `[{"name":"http","listen":8080,"service":80},{"name":"grpc","listen":9090,"service":9090}]`)

	cfg, err := LoadConfig(fakeEnv(vars))

	require.NoError(t, err)
	require.Equal(t, []PortConfig{
		{Name: "http", Listen: 8080, Service: 80},
		{Name: "grpc", Listen: 9090, Service: 9090},
	}, cfg.Ports)
}

func TestLoadConfig_UnnamedPort(t *testing.T) {
	cfg, err := LoadConfig(fakeEnv(envWith(EnvPorts, `[{"listen":8080,"service":80}]`)))

	require.NoError(t, err)
	require.Empty(t, cfg.Ports[0].Name)
}

func TestLoadConfig_Overrides(t *testing.T) {
	vars := validEnv()
	vars[EnvHealthPort] = "7777"
	vars[EnvMaxHops] = "2"
	vars[EnvRoutes] = "alice:alice-dev"
	vars[EnvRoutesDir] = "/etc/okteto/divert-routes"

	cfg, err := LoadConfig(fakeEnv(vars))

	require.NoError(t, err)
	require.Equal(t, 7777, cfg.HealthPort)
	require.Equal(t, 2, cfg.MaxHops)
	require.Equal(t, "alice:alice-dev", cfg.Routes)
	require.Equal(t, "/etc/okteto/divert-routes", cfg.RoutesDir)
}

func TestLoadConfig_TrimsWhitespace(t *testing.T) {
	vars := validEnv()
	vars[EnvServiceName] = "  api  "
	vars[EnvPorts] = "  " + vars[EnvPorts] + "  "

	cfg, err := LoadConfig(fakeEnv(vars))

	require.NoError(t, err)
	require.Equal(t, "api", cfg.Service)
	require.Len(t, cfg.Ports, 1)
}

// Readiness must not shadow a proxied path, so a collision with any proxied port — not just
// the first — is a configuration error.
func TestLoadConfig_RejectsAHealthPortThatShadowsASecondProxiedPort(t *testing.T) {
	vars := envWith(EnvPorts, `[{"listen":8080,"service":80},{"listen":9191,"service":9191}]`)

	_, err := LoadConfig(fakeEnv(vars))

	require.ErrorContains(t, err, "readiness must not shadow a proxied path")
}

func TestLoadConfig_Errors(t *testing.T) {
	tests := []struct {
		vars            map[string]string
		name            string
		expectedMessage string
	}{
		{
			name:            "missing service name",
			vars:            envWithout(EnvServiceName),
			expectedMessage: "SERVICE_NAME is required",
		},
		{
			name:            "blank service name",
			vars:            envWith(EnvServiceName, "   "),
			expectedMessage: "SERVICE_NAME is required",
		},
		{
			name:            "missing shared namespace",
			vars:            envWithout(EnvSharedNamespace),
			expectedMessage: "SHARED_NAMESPACE is required",
		},
		{
			name:            "missing baseline host",
			vars:            envWithout(EnvBaselineHost),
			expectedMessage: "BASELINE_HOST is required",
		},
		{
			name:            "baseline host carrying a port",
			vars:            envWith(EnvBaselineHost, "api-baseline.staging.svc.cluster.local:80"),
			expectedMessage: "must be a hostname without a port",
		},
		{
			name:            "missing ports",
			vars:            envWithout(EnvPorts),
			expectedMessage: "PORTS is required",
		},
		{
			name:            "ports that are not JSON",
			vars:            envWith(EnvPorts, "8080:80"),
			expectedMessage: "is not a valid JSON port list",
		},
		{
			name:            "an empty port list",
			vars:            envWith(EnvPorts, "[]"),
			expectedMessage: "PORTS lists no ports",
		},
		{
			name:            "a listen port out of range",
			vars:            envWith(EnvPorts, `[{"listen":0,"service":80}]`),
			expectedMessage: "PORTS listen port 0 is out of the 1-65535 range",
		},
		{
			name:            "a service port out of range",
			vars:            envWith(EnvPorts, `[{"listen":8080,"service":70000}]`),
			expectedMessage: "PORTS service port 70000 is out of the 1-65535 range",
		},
		{
			name:            "the same listen port twice",
			vars:            envWith(EnvPorts, `[{"listen":8080,"service":80},{"listen":8080,"service":90}]`),
			expectedMessage: "lists the listen port 8080 twice",
		},
		{
			name:            "non numeric health port",
			vars:            envWith(EnvHealthPort, "http"),
			expectedMessage: `HEALTH_PORT "http" is not a number`,
		},
		{
			name:            "health port out of range",
			vars:            envWith(EnvHealthPort, "65536"),
			expectedMessage: "HEALTH_PORT port 65536 is out of the 1-65535 range",
		},
		{
			name:            "health port shadowing the proxied port",
			vars:            envWith(EnvHealthPort, "8080"),
			expectedMessage: "readiness must not shadow a proxied path",
		},
		{
			name:            "non numeric max hops",
			vars:            envWith(EnvMaxHops, "many"),
			expectedMessage: `MAX_HOPS "many" is not a number`,
		},
		{
			name:            "zero max hops",
			vars:            envWith(EnvMaxHops, "0"),
			expectedMessage: `MAX_HOPS "0" must be greater than zero`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfig(fakeEnv(tt.vars))

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedMessage)
			require.Nil(t, cfg)
		})
	}
}
