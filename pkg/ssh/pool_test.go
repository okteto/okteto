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

package ssh

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
)

type deadlineRecordingConn struct {
	net.Conn
	deadlines []time.Time
}

func (c *deadlineRecordingConn) SetDeadline(deadline time.Time) error {
	c.deadlines = append(c.deadlines, deadline)
	return nil
}

func TestSetConnectionDeadlineUsesEarliestBound(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	recording := &deadlineRecordingConn{Conn: client}

	contextDeadline := time.Now().Add(200 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), contextDeadline)
	defer cancel()
	if err := setConnectionDeadline(ctx, recording, time.Second); err != nil {
		t.Fatal(err)
	}
	if len(recording.deadlines) != 1 || !recording.deadlines[0].Equal(contextDeadline) {
		t.Fatalf("deadline = %v, want context deadline %v", recording.deadlines, contextDeadline)
	}
}

func TestSetConnectionDeadlineUsesConfiguredTimeout(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	recording := &deadlineRecordingConn{Conn: client}
	before := time.Now()
	if err := setConnectionDeadline(context.Background(), recording, 200*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if len(recording.deadlines) != 1 {
		t.Fatalf("recorded %d deadlines, want 1", len(recording.deadlines))
	}
	if recording.deadlines[0].Before(before.Add(150*time.Millisecond)) || recording.deadlines[0].After(before.Add(300*time.Millisecond)) {
		t.Fatalf("deadline %v does not reflect a 200ms timeout from %v", recording.deadlines[0], before)
	}
}

func TestSetConnectionDeadlineUsesConfiguredTimeoutWhenEarlier(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	recording := &deadlineRecordingConn{Conn: client}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	before := time.Now()
	if err := setConnectionDeadline(ctx, recording, 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if len(recording.deadlines) != 1 || recording.deadlines[0].Before(before.Add(50*time.Millisecond)) || recording.deadlines[0].After(before.Add(250*time.Millisecond)) {
		t.Fatalf("deadline = %v, want configured timeout before context deadline", recording.deadlines)
	}
}

func TestSetConnectionDeadlineAllowsUnboundedConnection(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	recording := &deadlineRecordingConn{Conn: client}
	if err := setConnectionDeadline(context.Background(), recording, 0); err != nil {
		t.Fatal(err)
	}
	if len(recording.deadlines) != 0 {
		t.Fatalf("unbounded connection received deadlines: %v", recording.deadlines)
	}
}

func TestSetConnectionDeadlineRejectsCancelledContext(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	recording := &deadlineRecordingConn{Conn: client}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := setConnectionDeadline(ctx, recording, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("setConnectionDeadline() error = %v, want context.Canceled", err)
	}
	if len(recording.deadlines) != 0 {
		t.Fatalf("cancelled context set deadlines: %v", recording.deadlines)
	}
}

func TestInterruptConnectionOnCancelUnblocksStalledIO(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	disarm := interruptConnectionOnCancel(ctx, client)
	cancel()
	disarm()

	if _, err := client.Read(make([]byte, 1)); err == nil {
		t.Fatal("cancelled context did not interrupt stalled connection I/O")
	}
}

func TestInterruptConnectionDisarmPreventsLaterDeadline(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	recording := &deadlineRecordingConn{Conn: client}
	ctx, cancel := context.WithCancel(context.Background())
	disarm := interruptConnectionOnCancel(ctx, recording)
	disarm()
	cancel()
	if len(recording.deadlines) != 0 {
		t.Fatalf("disarmed cancellation set deadlines: %v", recording.deadlines)
	}
}

func TestSSHDialSinksRejectUnsafeAddressesBeforeNetworkAccess(t *testing.T) {
	t.Parallel()

	for _, address := range []string{"0.0.0.0:2222", "[::]:2222", "localhost:2222", "ssh.example.com:2222", "192.0.2.1:2222"} {
		if _, err := startPool(context.Background(), address, &cryptossh.ClientConfig{}); err == nil {
			t.Fatalf("startPool accepted unsafe address %q", address)
		} else if !strings.Contains(err.Error(), "refusing unsafe SSH server address") {
			t.Fatalf("startPool(%q) reached network setup instead of endpoint validation: %v", address, err)
		}
	}

	for _, host := range []string{"ssh.example.com", "", "192.0.2.1"} {
		if err := Exec(context.Background(), host, 2222, false, bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{}, []string{"true"}); err == nil {
			t.Fatalf("Exec accepted unsafe host %q", host)
		}
	}
}

func TestLocalSSHAddressCanonicalizesCompatibleInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		iface string
		want  string
	}{
		{iface: "localhost", want: "127.0.0.1:2222"},
		{iface: "0.0.0.0", want: "127.0.0.1:2222"},
		{iface: "::", want: "[::1]:2222"},
		{iface: "127.0.0.1", want: "127.0.0.1:2222"},
	}
	for _, tt := range tests {
		got, err := localSSHAddress(tt.iface, 2222)
		if err != nil {
			t.Fatalf("localSSHAddress(%q) error = %v", tt.iface, err)
		}
		if got != tt.want {
			t.Fatalf("localSSHAddress(%q) = %q, want %q", tt.iface, got, tt.want)
		}
	}
	for _, iface := range []string{"", "ssh.example.com", "192.0.2.1"} {
		if _, err := localSSHAddress(iface, 2222); err == nil {
			t.Fatalf("localSSHAddress(%q) accepted an unsafe endpoint", iface)
		}
	}
}
