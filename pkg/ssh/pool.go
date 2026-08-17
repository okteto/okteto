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

package ssh

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/okteto/okteto/internal/sshtransport"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"golang.org/x/crypto/ssh"
)

const (
	defaultRetries = 5
)

type pool struct {
	client  *ssh.Client
	ka      time.Duration
	stopped atomic.Bool
}

func startPool(ctx context.Context, serverAddr string, config *ssh.ClientConfig) (*pool, error) {
	return startPoolWithContexts(ctx, ctx, serverAddr, config)
}

func startPoolWithContexts(lifetimeCtx, connectionCtx context.Context, serverAddr string, config *ssh.ClientConfig) (*pool, error) {
	host, rawPort, err := net.SplitHostPort(serverAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid SSH server address %q: %w", serverAddr, err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return nil, fmt.Errorf("invalid SSH server port %q", rawPort)
	}
	serverAddr, err = sshtransport.LocalAddress(host, port)
	if err != nil {
		return nil, fmt.Errorf("refusing unsafe SSH server address: %w", err)
	}

	p := &pool{
		ka: 10 * time.Second,
	}

	client, err := start(connectionCtx, serverAddr, config, p.ka)
	if err != nil {
		return nil, err
	}

	p.client = client
	go p.keepAlive(lifetimeCtx)

	return p, nil
}

func start(ctx context.Context, serverAddr string, config *ssh.ClientConfig, keepAlive time.Duration) (*ssh.Client, error) {
	conn, err := getTCPConnection(ctx, serverAddr, keepAlive)
	if err != nil {
		return nil, fmt.Errorf("ssh getTCPConnection: %w", err)
	}
	if err := setConnectionDeadline(ctx, conn, config.Timeout); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh set handshake deadline: %w", err)
	}
	disarmCancellation := interruptConnectionOnCancel(ctx, conn)
	defer disarmCancellation()
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, serverAddr, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh NewClientConn: %w", err)
	}

	client := ssh.NewClient(clientConn, chans, reqs)

	if _, _, err := client.SendRequest("dev.okteto.com/ping", true, []byte("pong")); err != nil {
		client.Close()
		return nil, fmt.Errorf("ssh connection ping failed: %w", err)
	}
	disarmCancellation()
	if err := ctx.Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ssh connection setup cancelled: %w", err)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		client.Close()
		return nil, fmt.Errorf("ssh clear handshake deadline: %w", err)
	}

	oktetoLog.Infof("ssh ping to %s was successful", serverAddr)

	return client, nil
}

func (p *pool) keepAlive(ctx context.Context) {
	t := time.NewTicker(p.ka)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err != nil {
				if err != context.Canceled {
					oktetoLog.Infof("ssh pool keep alive completed with error: %s", err)
				}
			}

			return
		case <-t.C:
			if p.stopped.Load() {
				return
			}

			if _, _, err := p.client.SendRequest("dev.okteto.com/keepalive", true, nil); err != nil {
				oktetoLog.Infof("failed to send SSH keepalive: %s", err)
			}
		}
	}
}

func (p *pool) get(address string) (net.Conn, error) {
	c, err := p.client.Dial("tcp", address)
	return c, err
}

func (p *pool) getListener(address string) (net.Listener, error) {
	l, err := p.client.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to start ssh listener on %s: %w", address, err)
	}

	return l, nil
}

func getTCPConnection(ctx context.Context, serverAddr string, keepAlive time.Duration) (net.Conn, error) {
	c, err := getConn(ctx, serverAddr, defaultRetries)
	if err != nil {
		return nil, err
	}

	if err := c.(*net.TCPConn).SetKeepAlive(true); err != nil {
		c.Close()
		return nil, err
	}

	if err := c.(*net.TCPConn).SetKeepAlivePeriod(keepAlive); err != nil {
		c.Close()
		return nil, err
	}

	return c, nil
}

func getConn(ctx context.Context, serverAddr string, retries int) (net.Conn, error) {
	var lastErr error
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for i := 0; i < retries; i++ {
		d := net.Dialer{}
		c, err := d.DialContext(ctx, "tcp", serverAddr)
		if err == nil {
			return c, nil
		}

		lastErr = err
		select {
		case <-t.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, lastErr
}

func setConnectionDeadline(ctx context.Context, conn net.Conn, timeout time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if contextDeadline, ok := ctx.Deadline(); ok && (deadline.IsZero() || contextDeadline.Before(deadline)) {
		deadline = contextDeadline
	}
	if deadline.IsZero() {
		return nil
	}
	return conn.SetDeadline(deadline)
}

func interruptConnectionOnCancel(ctx context.Context, conn net.Conn) func() {
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = conn.SetDeadline(time.Now())
		close(done)
	})
	var once sync.Once
	return func() {
		once.Do(func() {
			if !stop() {
				<-done
			}
		})
	}
}

func (p *pool) stop() {
	if !p.stopped.CompareAndSwap(false, true) {
		return
	}
	if p.client == nil {
		return
	}
	if err := p.client.Close(); err != nil {
		if !oktetoErrors.IsClosedNetwork(err) {
			oktetoLog.Infof("failed to close SSH pool: %s", err)
		}
	}
}
