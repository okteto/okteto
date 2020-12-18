// Copyright 2020 The Okteto Authors
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

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"golang.org/x/crypto/ssh"
)

type pool struct {
	ka      time.Duration
	client  *ssh.Client
	stopped bool
}

func startPool(ctx context.Context, serverAddr string, config *ssh.ClientConfig) (*pool, error) {
	p := &pool{
		ka:      30 * time.Second,
		stopped: false,
	}

	var err error
	var client *ssh.Client
	t := time.NewTicker(500 * time.Millisecond)

	for i := 0; i < 10; i++ {
		client, err = start(ctx, serverAddr, config, p.ka)
		if err == nil {
			break
		}

		log.Infof("failed to establish SSH connection with your development container: %s", err)
		<-t.C
	}

	if err != nil {
		return nil, errors.ErrSSHConnectError
	}

	p.client = client
	go p.keepAlive(ctx)

	return p, nil
}

func start(ctx context.Context, serverAddr string, config *ssh.ClientConfig, keepAlive time.Duration) (*ssh.Client, error) {
	clientConn, chans, reqs, err := retryNewClientConn(ctx, serverAddr, config, keepAlive)
	if err != nil {
		return nil, fmt.Errorf("failed to create ssh client connection: %w", err)
	}

	client := ssh.NewClient(clientConn, chans, reqs)

	r, c, err := client.SendRequest("dev.okteto.com/ping", true, []byte("pong"))
	if err != nil {
		return nil, fmt.Errorf("ssh connection ping failed: %w", err)
	}

	log.Infof("ssh ping to %s was successful: %t %c", serverAddr, r, string(c))

	return client, nil
}

func retryNewClientConn(ctx context.Context, addr string, conf *ssh.ClientConfig, keepAlive time.Duration) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	ticker := time.NewTicker(300 * time.Millisecond)
	to := config.GetTimeout() / 10 // 3 seconds
	timeout := time.Now().Add(to)

	log.Infof("waiting for ssh connection to %s to be ready", addr)
	for i := 0; ; i++ {
		conn, err := getTCPConnection(ctx, addr, keepAlive)
		if err == nil {
			clientConn, chans, reqs, errConn := ssh.NewClientConn(conn, addr, conf)
			if errConn == nil {
				log.Infof("ssh connection to %s is ready", addr)
				return clientConn, chans, reqs, nil
			}
			err = errConn
		}

		log.Infof("ssh connection to %s is not yet ready: %s", addr, err)

		if time.Now().After(timeout) {
			return nil, nil, nil, fmt.Errorf("ssh connection to %s wasn't ready after %s: %s", addr, to.String(), err)
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			log.Infof("ssh.retryNewClientConn cancelled")
			return nil, nil, nil, fmt.Errorf("ssh.retryNewClientConn cancelled")
		}
	}
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
					log.Infof("ssh pool keep alive completed with error: %s", err)
				}
			}

			return
		case <-t.C:
			if p.stopped {
				return
			}

			if _, _, err := p.client.SendRequest("dev.okteto.com/keepalive", true, nil); err != nil {
				log.Infof("failed to send SSH keepalive: %s", err)
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
	c, err := getConn(ctx, serverAddr, 3)
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

func getConn(ctx context.Context, serverAddr string, maxRetries int) (net.Conn, error) {
	var lastErr error
	t := time.NewTicker(100 * time.Millisecond)
	for i := 0; i < 5; i++ {
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
		if !errors.IsClosedNetwork(err) {
			log.Infof("failed to close SSH pool: %s", err)
		}
	}
}
