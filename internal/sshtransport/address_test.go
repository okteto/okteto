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

package sshtransport

import (
	"net"
	"net/netip"
	"strconv"
	"testing"
)

func TestPlan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		iface       string
		wantBind    string
		wantDial    string
		wantAddress string
		port        int
		requested   bool
		wantErr     bool
	}{
		{name: "localhost", iface: "localhost", port: 2222, wantBind: "127.0.0.1", wantDial: "127.0.0.1", wantAddress: "127.0.0.1:2222"},
		{name: "localhost case", iface: "LOCALHOST", port: 2222, wantBind: "127.0.0.1", wantDial: "127.0.0.1", wantAddress: "127.0.0.1:2222"},
		{name: "IPv4 wildcard", iface: "0.0.0.0", port: 2222, wantBind: "0.0.0.0", wantDial: "127.0.0.1", wantAddress: "127.0.0.1:2222"},
		{name: "IPv6 wildcard", iface: "::", port: 2222, wantBind: "::", wantDial: "::1", wantAddress: "[::1]:2222"},
		{name: "IPv4 loopback", iface: "127.0.0.2", port: 2222, wantBind: "127.0.0.2", wantDial: "127.0.0.2", wantAddress: "127.0.0.2:2222"},
		{name: "IPv6 loopback", iface: "::1", port: 2222, wantBind: "::1", wantDial: "::1", wantAddress: "[::1]:2222"},
		{name: "specific IPv4", iface: "192.0.2.10", port: 2222, wantBind: "192.0.2.10", wantDial: "192.0.2.10", wantAddress: "192.0.2.10:2222"},
		{name: "specific IPv6", iface: "2001:db8::10", port: 2222, wantBind: "2001:db8::10", wantDial: "2001:db8::10", wantAddress: "[2001:db8::10]:2222"},
		{name: "mapped IPv4", iface: "::ffff:127.0.0.1", port: 2222, wantBind: "127.0.0.1", wantDial: "127.0.0.1", wantAddress: "127.0.0.1:2222"},
		{name: "sloppy IPv4", iface: "127.000.000.001", port: 2222, wantBind: "127.0.0.1", wantDial: "127.0.0.1", wantAddress: "127.0.0.1:2222"},
		{name: "automatic", iface: "localhost", port: 0, requested: true, wantBind: "127.0.0.1", wantDial: "127.0.0.1", wantAddress: "127.0.0.1:0"},
		{name: "zero cannot be dialed", iface: "localhost", port: 0, wantErr: true},
		{name: "negative port", iface: "localhost", port: -1, requested: true, wantErr: true},
		{name: "large port", iface: "localhost", port: 65536, requested: true, wantErr: true},
		{name: "empty interface", iface: "", port: 2222, wantErr: true},
		{name: "hostname", iface: "ssh.example.com", port: 2222, wantErr: true},
		{name: "zone", iface: "fe80::1%lo0", port: 2222, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got Endpoint
			var err error
			if tt.requested {
				got, err = PlanRequested(tt.iface, tt.port)
			} else {
				got, err = Plan(tt.iface, tt.port)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("endpoint plan unexpectedly succeeded: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.BindHost() != tt.wantBind || got.DialHost() != tt.wantDial || got.Address() != tt.wantAddress {
				t.Fatalf("Plan(%q, %d) = bind %q, dial %q, address %q; want %q, %q, %q", tt.iface, tt.port, got.BindHost(), got.DialHost(), got.Address(), tt.wantBind, tt.wantDial, tt.wantAddress)
			}
			assertSafeEndpoint(t, got, tt.port)
		})
	}
}

func TestConcreteAddress(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		host    string
		port    int
		want    string
		wantErr bool
	}{
		{host: "127.0.0.1", port: 1, want: "127.0.0.1:1"},
		{host: "::1", port: 65535, want: "[::1]:65535"},
		{host: "192.0.2.10", port: 2222, want: "192.0.2.10:2222"},
		{host: "localhost", port: 2222, wantErr: true},
		{host: "0.0.0.0", port: 2222, wantErr: true},
		{host: "::", port: 2222, wantErr: true},
		{host: "127.0.0.1", port: 0, wantErr: true},
		{host: "127.0.0.1", port: 65536, wantErr: true},
	} {
		got, err := ConcreteAddress(tt.host, tt.port)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("ConcreteAddress(%q, %d) = %q, want error", tt.host, tt.port, got)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Fatalf("ConcreteAddress(%q, %d) = %q, %v; want %q, nil", tt.host, tt.port, got, err, tt.want)
		}
	}
}

func TestConcreteRequestedAddress(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		host    string
		port    int
		want    string
		wantErr bool
	}{
		{host: "127.0.0.1", port: 0, want: "127.0.0.1:0"},
		{host: "::1", port: 0, want: "[::1]:0"},
		{host: "127.0.0.1", port: 65535, want: "127.0.0.1:65535"},
		{host: "localhost", port: 0, wantErr: true},
		{host: "0.0.0.0", port: 0, wantErr: true},
		{host: "::", port: 0, wantErr: true},
		{host: "127.0.0.1", port: -1, wantErr: true},
		{host: "127.0.0.1", port: 65536, wantErr: true},
	} {
		got, err := ConcreteRequestedAddress(tt.host, tt.port)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("ConcreteRequestedAddress(%q, %d) = %q, want error", tt.host, tt.port, got)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Fatalf("ConcreteRequestedAddress(%q, %d) = %q, %v; want %q, nil", tt.host, tt.port, got, err, tt.want)
		}
	}
}

func TestIsLocalAddressRecognizesLoopbackWithoutInterfaceLookup(t *testing.T) {
	t.Parallel()

	for _, host := range []string{"127.0.0.1", "127.0.0.2", "::1"} {
		local, err := IsLocalAddress(host)
		if err != nil || !local {
			t.Fatalf("IsLocalAddress(%q) = %v, %v; want true, nil", host, local, err)
		}
	}
	for _, host := range []string{"localhost", "0.0.0.0", "::", "ssh.example.com"} {
		if local, err := IsLocalAddress(host); err == nil || local {
			t.Fatalf("IsLocalAddress(%q) = %v, %v; want false, error", host, local, err)
		}
	}
}

func TestLocalAddress(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		host string
		want string
	}{
		{host: "127.0.0.1", want: "127.0.0.1:2222"},
		{host: "::1", want: "[::1]:2222"},
	} {
		got, err := LocalAddress(tt.host, 2222)
		if err != nil || got != tt.want {
			t.Fatalf("LocalAddress(%q, 2222) = %q, %v; want %q, nil", tt.host, got, err, tt.want)
		}
	}

	for _, tt := range []struct {
		host string
		port int
	}{
		{host: "localhost", port: 2222},
		{host: "0.0.0.0", port: 2222},
		{host: "::", port: 2222},
		{host: "192.0.2.10", port: 2222},
		{host: "127.0.0.1", port: 0},
	} {
		if got, err := LocalAddress(tt.host, tt.port); err == nil {
			t.Fatalf("LocalAddress(%q, %d) = %q, want error", tt.host, tt.port, got)
		}
	}
}

func FuzzPlanRequested(f *testing.F) {
	for _, seed := range []struct {
		iface string
		port  int
	}{{"localhost", 0}, {"0.0.0.0", 2222}, {"::", 65535}, {"192.0.2.10", 22}, {"bad.example", -1}} {
		f.Add(seed.iface, seed.port)
	}

	f.Fuzz(func(t *testing.T, iface string, port int) {
		endpoint, err := PlanRequested(iface, port)
		if err != nil {
			return
		}
		assertSafeEndpoint(t, endpoint, port)
	})
}

func assertSafeEndpoint(t *testing.T, endpoint Endpoint, wantPort int) {
	t.Helper()

	host, port, err := net.SplitHostPort(endpoint.Address())
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", endpoint.Address(), err)
	}
	dialIP, err := netip.ParseAddr(host)
	if err != nil {
		t.Fatalf("ParseAddr(%q): %v", host, err)
	}
	if dialIP.IsUnspecified() {
		t.Fatalf("endpoint returned unspecified dial host %q", host)
	}
	if port != strconv.Itoa(wantPort) {
		t.Fatalf("endpoint returned port %q, want %d", port, wantPort)
	}

	bindIP, err := netip.ParseAddr(endpoint.BindHost())
	if err != nil {
		t.Fatalf("ParseAddr(%q): %v", endpoint.BindHost(), err)
	}
	if bindIP != dialIP && (!bindIP.IsUnspecified() || bindIP.Is4() != dialIP.Is4()) {
		t.Fatalf("bind host %q does not cover dial host %q", endpoint.BindHost(), endpoint.DialHost())
	}
}
