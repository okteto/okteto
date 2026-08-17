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

package forward

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/tools/portforward"
)

type contractDialer struct {
	connection httpstream.Connection
}

func (d *contractDialer) Dial(...string) (httpstream.Connection, string, error) {
	return d.connection, portforward.PortForwardProtocolV1Name, nil
}

type contractConnection struct {
	closeOnce sync.Once
	closed    chan bool
}

func (*contractConnection) CreateStream(http.Header) (httpstream.Stream, error) {
	return nil, fmt.Errorf("unexpected stream creation")
}

func (c *contractConnection) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}

func (c *contractConnection) CloseChan() <-chan bool           { return c.closed }
func (*contractConnection) RemoveStreams(...httpstream.Stream) {}
func (*contractConnection) SetIdleTimeout(time.Duration)       {}

func TestClientGoHoldsAutomaticallyAllocatedPortUntilStop(t *testing.T) {
	connection := &contractConnection{closed: make(chan bool)}
	stop := make(chan struct{})
	ready := make(chan struct{})
	forwarder, err := portforward.NewOnAddresses(
		&contractDialer{connection: connection},
		[]string{"127.0.0.1"},
		[]string{"0:2222"},
		stop,
		ready,
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() { result <- forwarder.ForwardPorts() }()
	select {
	case <-ready:
	case err := <-result:
		t.Fatalf("ForwardPorts returned before readiness: %v", err)
	case <-time.After(time.Second):
		t.Fatal("client-go did not report port-forward readiness")
	}

	ports, err := forwarder.GetPorts()
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 || ports[0].Local == 0 || ports[0].Remote != 2222 {
		t.Fatalf("GetPorts() = %+v, want one concrete local port mapped to 2222", ports)
	}
	address := net.JoinHostPort("127.0.0.1", fmt.Sprint(ports[0].Local))
	if competing, err := net.Listen("tcp4", address); err == nil {
		competing.Close()
		t.Fatalf("client-go did not hold automatically allocated address %s", address)
	}

	close(stop)
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("ForwardPorts() after Stop = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("client-go did not release the listener after Stop")
	}
	rebound, err := net.Listen("tcp4", address)
	if err != nil {
		t.Fatalf("client-go did not release automatically allocated address %s: %v", address, err)
	}
	rebound.Close()
}
