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
	"errors"
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestAddressDoesNotReachOppositeFamilyDecoy(t *testing.T) {
	legitimate, err := net.Listen("tcp4", net.JoinHostPort(LoopbackHost, "0"))
	if err != nil {
		t.Fatalf("listen on IPv4 loopback: %v", err)
	}
	defer legitimate.Close()

	port := legitimate.Addr().(*net.TCPAddr).Port
	decoy, err := net.Listen("tcp6", net.JoinHostPort("::1", strconv.Itoa(port)))
	if err != nil {
		t.Skipf("IPv6 loopback is unavailable: %v", err)
	}
	defer decoy.Close()

	endpoint, err := Plan("0.0.0.0", port)
	if err != nil {
		t.Fatal(err)
	}
	address := endpoint.Address()
	dialer := net.Dialer{Timeout: time.Second}
	client, err := dialer.Dial("tcp", address)
	if err != nil {
		t.Fatalf("dial legitimate endpoint: %v", err)
	}
	defer client.Close()

	if err := legitimate.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	server, err := legitimate.Accept()
	if err != nil {
		t.Fatalf("legitimate listener did not receive the connection: %v", err)
	}
	defer server.Close()

	payload := []byte("okteto-ssh-endpoint")
	if _, err := client.Write(payload); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(server, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("legitimate listener got %q, want %q", got, payload)
	}

	if err := decoy.(*net.TCPListener).SetDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	decoyConnection, err := decoy.Accept()
	if err == nil {
		decoyConnection.Close()
		t.Fatal("opposite-family decoy received the SSH connection")
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("decoy accept returned an unexpected error: %v", err)
	}
}

func TestIPv6AddressDoesNotReachOppositeFamilyDecoy(t *testing.T) {
	legitimate, err := net.Listen("tcp6", net.JoinHostPort("::1", "0"))
	if err != nil {
		t.Skipf("IPv6 loopback is unavailable: %v", err)
	}
	defer legitimate.Close()

	port := legitimate.Addr().(*net.TCPAddr).Port
	decoy, err := net.Listen("tcp4", net.JoinHostPort(LoopbackHost, strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("listen on IPv4 decoy: %v", err)
	}
	defer decoy.Close()

	endpoint, err := Plan("::", port)
	if err != nil {
		t.Fatal(err)
	}
	dialer := net.Dialer{Timeout: time.Second}
	client, err := dialer.Dial("tcp", endpoint.Address())
	if err != nil {
		t.Fatalf("dial legitimate endpoint: %v", err)
	}
	defer client.Close()

	if err := legitimate.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	server, err := legitimate.Accept()
	if err != nil {
		t.Fatalf("legitimate listener did not receive the connection: %v", err)
	}
	server.Close()

	if err := decoy.(*net.TCPListener).SetDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	decoyConnection, err := decoy.Accept()
	if err == nil {
		decoyConnection.Close()
		t.Fatal("opposite-family decoy received the SSH connection")
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("decoy accept returned an unexpected error: %v", err)
	}
}
