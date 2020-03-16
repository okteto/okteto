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

	"github.com/okteto/okteto/pkg/log"
	"golang.org/x/crypto/ssh"
)

type pool struct {
	ka     time.Duration
	ctx    context.Context
	cancel func()

	client *ssh.Client
}

func startPool(ctx context.Context, serverAddr string, config *ssh.ClientConfig) (*pool, error) {
	c, cancel := context.WithCancel(ctx)
	p := &pool{
		ka:     30 * time.Second,
		ctx:    c,
		cancel: cancel,
	}

	conn, err := getTCPConnection(serverAddr, p.ka)
	if err != nil {
		return nil, fmt.Errorf("failed to establish a tcp connection: %s", err)
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(conn, serverAddr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create ssh connection: %w", err)
	}

	client := ssh.NewClient(clientConn, chans, reqs)
	if err != nil {
		return nil, fmt.Errorf("failed to start ssh client: %w", err)
	}

	p.client = client
	go p.keepAlive()

	return p, nil
}

func (p *pool) keepAlive() {
	t := time.NewTicker(p.ka)
	defer t.Stop()
	for {
		select {
		case <-p.ctx.Done():
			log.Infof("ssh pool keep alive completed")
			return
		case <-t.C:
			if _, _, err := p.client.SendRequest("dev.okteto.com/keepalive", true, nil); err != nil {
				log.Infof("failed to send SSH keepalive: %s", err)
			}
		}
	}
}

func (p *pool) get(address string) (net.Conn, error) {
	c, err := p.client.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to start ssh connection: %w", err)
	}

	return c, nil
}

func (p *pool) getListener(address string) (net.Listener, error) {
	l, err := p.client.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to start ssh listener on %s: %w", address, err)
	}

	return l, nil
}

func getTCPConnection(serverAddr string, keepAlive time.Duration) (net.Conn, error) {
	c, err := net.Dial("tcp", serverAddr)
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
