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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Environment variables read by the router. They are exported because whatever writes the
// router's pod spec has to set exactly these names; a private copy on the writing side
// would drift silently.
const (
	EnvServiceName     = "SERVICE_NAME"
	EnvSharedNamespace = "SHARED_NAMESPACE"
	EnvBaselineHost    = "BASELINE_HOST"
	EnvPorts           = "PORTS"
	EnvHealthPort      = "HEALTH_PORT"
	EnvRoutes          = "ROUTES"
	EnvRoutesDir       = "ROUTES_DIR"
	EnvMaxHops         = "MAX_HOPS"
)

const (
	minPort = 1
	maxPort = 65535

	// defaultHealthPort is where readiness is served. It is deliberately not a listen
	// port: the router must forward every path it receives untouched, so an application
	// that exposes its own /healthz has to keep winning that path.
	defaultHealthPort = 9191

	// defaultMaxHops bounds how many diverted proxies a single request may traverse before
	// it is rejected as a loop. Real call chains that cross more than a handful of diverted
	// services are far rarer than a service diverted onto itself.
	defaultMaxHops = 5
)

// PortConfig is one port of the diverted Service.
//
// Listen and Service are different numbers and both are needed. The router binds Listen,
// the original Service's targetPort, so callers reaching the swapped Service arrive
// unchanged. It dials Service, the Service's port, because it forwards to a Service rather
// than to a pod. Conflating the two is invisible only while they happen to be equal.
type PortConfig struct {
	// Name is the *container* port name the Service targets, empty when it targets a number
	// instead. It has to be declared on the router's container port, because a Service
	// addressing its pods by name resolves that name against them.
	//
	// This is deliberately not the Service port's own name: that one may be up to 63
	// characters, while a container port name is capped at 15, so copying it across could
	// produce a pod that will not validate.
	Name string `json:"name,omitempty"`

	// Listen is the port the router binds.
	Listen int `json:"listen"`

	// Service is the port to dial on the baseline and on the developer's copy.
	Service int `json:"service"`
}

// Config is the router's runtime configuration.
type Config struct {
	// Service is the name of the service being diverted. It is the same name in the shared
	// namespace and in every developer namespace, which is what makes the destination
	// address derivable from the routing key alone.
	Service string

	// SharedNamespace is the namespace holding the original Service and the baseline.
	SharedNamespace string

	// BaselineHost is the hostname — no port — of the `<service>-baseline` Service, which
	// carries the selector the original Service had before the swap. It is where every
	// non-diverted request goes. The port comes from whichever PortConfig served it.
	BaselineHost string

	// Routes is the raw route spec, `key1:namespace1,key2:namespace2`. Used only when
	// RoutesDir is unset.
	Routes string

	// RoutesDir is a directory holding one file per routing key, as a mounted ConfigMap
	// materialises it. When set it takes precedence over Routes, and the router picks up
	// changes to it without restarting.
	RoutesDir string

	// Ports is every port of the diverted Service. The router serves all of them, because a
	// Service that exposes two ports is unusable if a divert only carries one of them.
	Ports []PortConfig

	// HealthPort serves readiness, on its own listener so that every proxied port stays
	// transparent for every path, including the application's own health endpoints.
	HealthPort int

	// MaxHops is the loop-guard threshold.
	MaxHops int
}

// LoadConfig builds a Config from the environment. getenv is injected so the loader can be
// exercised without mutating process state.
func LoadConfig(getenv func(string) string) (*Config, error) {
	cfg := &Config{
		Service:         strings.TrimSpace(getenv(EnvServiceName)),
		SharedNamespace: strings.TrimSpace(getenv(EnvSharedNamespace)),
		BaselineHost:    strings.TrimSpace(getenv(EnvBaselineHost)),
		Routes:          getenv(EnvRoutes),
		RoutesDir:       strings.TrimSpace(getenv(EnvRoutesDir)),
	}

	if cfg.Service == "" {
		return nil, fmt.Errorf("%s is required", EnvServiceName)
	}
	if cfg.SharedNamespace == "" {
		return nil, fmt.Errorf("%s is required", EnvSharedNamespace)
	}
	if cfg.BaselineHost == "" {
		return nil, fmt.Errorf("%s is required", EnvBaselineHost)
	}
	if strings.Contains(cfg.BaselineHost, ":") {
		return nil, fmt.Errorf("%s %q must be a hostname without a port: the port comes from %s", EnvBaselineHost, cfg.BaselineHost, EnvPorts)
	}

	ports, err := parsePorts(getenv(EnvPorts))
	if err != nil {
		return nil, err
	}
	cfg.Ports = ports

	cfg.HealthPort, err = optionalPort(getenv, EnvHealthPort, defaultHealthPort)
	if err != nil {
		return nil, err
	}
	for _, port := range cfg.Ports {
		if port.Listen == cfg.HealthPort {
			return nil, fmt.Errorf("%s and a proxied port cannot both be %d: readiness must not shadow a proxied path", EnvHealthPort, cfg.HealthPort)
		}
	}

	cfg.MaxHops, err = optionalMaxHops(getenv)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// parsePorts decodes the JSON port list. JSON rather than a bespoke format because each
// entry carries three fields, one of them an optional name that may contain a dash.
func parsePorts(raw string) ([]PortConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%s is required", EnvPorts)
	}

	var ports []PortConfig
	if err := json.Unmarshal([]byte(raw), &ports); err != nil {
		return nil, fmt.Errorf("%s %q is not a valid JSON port list: %w", EnvPorts, raw, err)
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("%s lists no ports", EnvPorts)
	}

	seen := make(map[int]bool, len(ports))
	for _, port := range ports {
		if err := validatePort(EnvPorts+" listen", port.Listen); err != nil {
			return nil, err
		}
		if err := validatePort(EnvPorts+" service", port.Service); err != nil {
			return nil, err
		}
		if seen[port.Listen] {
			return nil, fmt.Errorf("%s lists the listen port %d twice", EnvPorts, port.Listen)
		}
		seen[port.Listen] = true
	}

	return ports, nil
}

func validatePort(name string, port int) error {
	if port < minPort || port > maxPort {
		return fmt.Errorf("%s port %d is out of the %d-%d range", name, port, minPort, maxPort)
	}

	return nil
}

func optionalPort(getenv func(string) string, name string, fallback int) (int, error) {
	raw := strings.TrimSpace(getenv(name))
	if raw == "" {
		return fallback, nil
	}

	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s %q is not a number: %w", name, raw, err)
	}
	if err := validatePort(name, port); err != nil {
		return 0, err
	}

	return port, nil
}

func optionalMaxHops(getenv func(string) string) (int, error) {
	raw := strings.TrimSpace(getenv(EnvMaxHops))
	if raw == "" {
		return defaultMaxHops, nil
	}

	maxHops, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s %q is not a number: %w", EnvMaxHops, raw, err)
	}
	if maxHops < 1 {
		return 0, fmt.Errorf("%s %q must be greater than zero", EnvMaxHops, raw)
	}

	return maxHops, nil
}
