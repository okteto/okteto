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

// Package sshtransport plans the local endpoint used by Okteto's SSH
// transport. It preserves the manifest's listener scope while ensuring every
// in-process SSH client uses a concrete numeric address covered by that bind.
package sshtransport

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	netutils "k8s.io/utils/net"
)

// LoopbackHost is the concrete local address used by the internal SSH tunnel.
const LoopbackHost = "127.0.0.1"

// Endpoint separates the address exposed by the local listener from the
// concrete address used by Okteto's own SSH clients.
type Endpoint struct {
	bindHost string
	dialHost string
	port     int
}

// Plan returns a validated endpoint with a concrete, non-wildcard dial host.
func Plan(iface string, port int) (Endpoint, error) {
	return plan(iface, port, false)
}

// PlanRequested is equivalent to Plan but permits port zero so client-go can
// choose and hold an available listener atomically.
func PlanRequested(iface string, port int) (Endpoint, error) {
	return plan(iface, port, true)
}

func plan(iface string, port int, allowZero bool) (Endpoint, error) {
	minimumPort := 1
	if allowZero {
		minimumPort = 0
	}
	if port < minimumPort || port > 65535 {
		return Endpoint{}, fmt.Errorf("invalid SSH tunnel port %d: must be between %d and 65535", port, minimumPort)
	}

	if strings.EqualFold(iface, "localhost") {
		// client-go expands localhost to both loopback families and reports ready
		// when either listener succeeds. Pinning both bind and dial to IPv4 avoids
		// dialing an occupied IPv4 port after only the IPv6 listener succeeded.
		// IPv6-only hosts can opt in explicitly with interface ::1.
		return Endpoint{bindHost: LoopbackHost, dialHost: LoopbackHost, port: port}, nil
	}

	ip, err := parseIP(iface)
	if err != nil {
		return Endpoint{}, fmt.Errorf("SSH bind interface %q must be localhost or a numeric IP address", iface)
	}
	bindHost := ip.String()
	dialHost := bindHost
	if ip.IsUnspecified() {
		if ip.Is4() {
			dialHost = LoopbackHost
		} else {
			dialHost = "::1"
		}
	}

	return Endpoint{bindHost: bindHost, dialHost: dialHost, port: port}, nil
}

// BindHost is the numeric address passed to client-go's listener.
func (e Endpoint) BindHost() string {
	return e.bindHost
}

// DialHost is the concrete numeric address used by Okteto's SSH clients.
func (e Endpoint) DialHost() string {
	return e.dialHost
}

// Address returns the concrete SSH dial address.
func (e Endpoint) Address() string {
	return net.JoinHostPort(e.dialHost, strconv.Itoa(e.port))
}

// DialIsLoopback reports whether the planned dial host is local loopback.
func (e Endpoint) DialIsLoopback() bool {
	ip, err := netip.ParseAddr(e.dialHost)
	return err == nil && ip.IsLoopback()
}

// ConcreteAddress validates and formats a numeric, non-wildcard dial target.
func ConcreteAddress(host string, port int) (string, error) {
	return concreteAddress(host, port, false)
}

// ConcreteRequestedAddress is equivalent to ConcreteAddress but permits port
// zero before client-go has allocated the listener.
func ConcreteRequestedAddress(host string, port int) (string, error) {
	return concreteAddress(host, port, true)
}

func concreteAddress(host string, port int, allowZero bool) (string, error) {
	minimumPort := 1
	if allowZero {
		minimumPort = 0
	}
	if port < minimumPort || port > 65535 {
		return "", fmt.Errorf("invalid SSH tunnel port %d: must be between %d and 65535", port, minimumPort)
	}

	ip, err := parseIP(host)
	if err != nil || ip.IsUnspecified() {
		return "", fmt.Errorf("SSH dial host %q must be a concrete numeric IP address", host)
	}

	return net.JoinHostPort(ip.Unmap().String(), strconv.Itoa(port)), nil
}

// IsLocalAddress reports whether host is assigned to this machine. Loopback
// addresses are accepted without consulting the host's interface table.
func IsLocalAddress(host string) (bool, error) {
	target, err := parseIP(host)
	if err != nil || target.IsUnspecified() {
		return false, fmt.Errorf("SSH dial host %q must be a concrete numeric IP address", host)
	}
	if target.IsLoopback() {
		return true, nil
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return false, fmt.Errorf("list local network interfaces: %w", err)
	}
	for _, iface := range interfaces {
		addresses, err := iface.Addrs()
		if err != nil {
			return false, fmt.Errorf("list addresses for interface %q: %w", iface.Name, err)
		}
		for _, address := range addresses {
			var localIP net.IP
			switch value := address.(type) {
			case *net.IPNet:
				localIP = value.IP
			case *net.IPAddr:
				localIP = value.IP
			}
			if localIP != nil && localIP.Equal(net.IP(target.AsSlice())) {
				return true, nil
			}
		}
	}

	return false, nil
}

func parseIP(value string) (netip.Addr, error) {
	if strings.Contains(value, "%") {
		return netip.Addr{}, fmt.Errorf("zoned IP addresses are not supported")
	}
	ip := netutils.ParseIPSloppy(value)
	if ip == nil {
		return netip.Addr{}, fmt.Errorf("invalid IP address")
	}
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}, fmt.Errorf("invalid IP address")
	}
	return address.Unmap(), nil
}

// LocalAddress validates a concrete SSH endpoint and proves its host belongs
// to this machine before returning the canonical dial address.
func LocalAddress(host string, port int) (string, error) {
	address, err := ConcreteAddress(host, port)
	if err != nil {
		return "", err
	}
	local, err := IsLocalAddress(host)
	if err != nil {
		return "", err
	}
	if !local {
		return "", fmt.Errorf("SSH dial host %q is not assigned to this machine", host)
	}
	return address, nil
}
