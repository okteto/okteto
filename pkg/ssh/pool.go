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
	"time"

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
	stopped bool
}

func startPool(ctx context.Context, serverAddr string, config *ssh.ClientConfig) (*pool, error) {
	p := &pool{
		ka:      10 * time.Second,
		stopped: false,
	}

	client, err := start(ctx, serverAddr, config, p.ka)
	if err != nil {
		return nil, err
	}

	p.client = client
	go p.keepAlive(ctx)

	return p, nil
}

func start(ctx context.Context, serverAddr string, config *ssh.ClientConfig, keepAlive time.Duration) (*ssh.Client, error) {
	conn, err := getTCPConnection(ctx, serverAddr, keepAlive)
	if err != nil {
		return nil, fmt.Errorf("ssh getTCPConnection: %w", err)
	}
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, serverAddr, config)
	if err != nil {
		return nil, fmt.Errorf("ssh NewClientConn: %w", err)
	}

	client := ssh.NewClient(clientConn, chans, reqs)

	if _, _, err := client.SendRequest("dev.okteto.com/ping", true, []byte("pong")); err != nil {
		return nil, fmt.Errorf("ssh connection ping failed: %w", err)
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
			if p.stopped {
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
		return nil, err
	}

	if err := c.(*net.TCPConn).SetKeepAlivePeriod(keepAlive); err != nil {
		return nil, err
	}

	return c, nil
}

func getConn(ctx context.Context, serverAddr string, retries int) (net.Conn, error) {
	var lastErr error
	t := time.NewTicker(100 * time.Millisecond)
	for i := 0; i < retries; i++ {
		d := net.Dialer{}
		c, err := d.DialContext(ctx, "tcp", serverAddr)
		if err == nil {
			return c, nil
		}

		lastErr = err
		<-t.C
	}

	return nil, lastErr
}

func (p *pool) stop() {
	p.stopped = true
	if err := p.client.Close(); err != nil {
		if !oktetoErrors.IsClosedNetwork(err) {
			oktetoLog.Infof("failed to close SSH pool: %s", err)
		}
	}
}
