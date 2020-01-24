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
	"io"
	"net"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"golang.org/x/crypto/ssh"
)

type reverse struct {
	localPort  int
	remotePort int
}

// ReverseManager handles the lifecycle of all the remote forwards
type ReverseManager struct {
	reverses map[int]*reverse
	ctx      context.Context
	sshUser  string
	sshHost  string
	sshPort  int
}

// NewReverseManager returns a newly initialized instance of RemoteReverseManager
func NewReverseManager(ctx context.Context, sshPort int) *ReverseManager {
	return &ReverseManager{
		ctx:      ctx,
		reverses: make(map[int]*reverse),
		sshUser:  "root",
		sshHost:  "localhost",
		sshPort:  sshPort,
	}
}

// Add initializes a remote forward
func (r *ReverseManager) Add(f *model.Reverse) error {

	localPort := f.Local
	remotePort := f.Remote

	if _, ok := r.reverses[localPort]; ok {
		return fmt.Errorf("port %d is already taken, please check your remote forward configuration", localPort)
	}

	r.reverses[localPort] = &reverse{localPort: localPort, remotePort: remotePort}
	return nil
}

// Start starts all the remote forwards as goroutines
func (r *ReverseManager) Start() error {
	log.Info("starting remote forward manager")

	// Connect to SSH remote server using serverEndpoint
	c := &ssh.ClientConfig{
		User: r.sshUser,
		// skipcq GSC-G106
		// Ignoring this issue since the remote server doesn't have a set identity, and it's already secured by the
		// port-forward tunnel to the kubernetes cluster.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshAddr := fmt.Sprintf("%s:%d", r.sshHost, r.sshPort)

	for _, rt := range r.reverses {
		go rt.startWithRetry(r.ctx, c, sshAddr)
	}

	return nil
}

func (r *reverse) startWithRetry(ctx context.Context, c *ssh.ClientConfig, sshAddr string) {
	log.Infof("starting remote forward tunnel %d->%d", r.remotePort, r.localPort)

	for {
		err := r.start(ctx, c, sshAddr)
		if err == nil {
			log.Infof("remote forward tunnel %d->%d exited", r.remotePort, r.localPort)
			return
		}

		log.Infof("remote forward tunnel %d->%d not connected, retrying: %s", r.remotePort, r.localPort, err)
		t := time.NewTicker(3 * time.Second)
		<-t.C
	}
}

func (r *reverse) start(ctx context.Context, c *ssh.ClientConfig, sshAddr string) error {
	log.Infof("starting remote forward tunnel %d->%d", r.remotePort, r.localPort)

	serverConn, err := ssh.Dial("tcp", sshAddr, c)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH host: %s", err)
	}

	// Listen on remote server port
	listener, err := serverConn.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", r.remotePort))
	if err != nil {
		return fmt.Errorf("failed open remote port %d: %s", r.remotePort, err)
	}

	defer listener.Close()

	// handle incoming connections on remote forward tunnel
	for {
		// listen on local port
		var d net.Dialer

		local, err := d.DialContext(ctx, "tcp", fmt.Sprintf("localhost:%d", r.localPort))
		if err != nil {
			return fmt.Errorf("failed to open local port %d: %s", r.remotePort, err)
		}

		client, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept traffic from remote port %d: %s", r.remotePort, err)
		}

		r.handleClient(client, local)
	}
}

func (r *reverse) handleClient(client net.Conn, local net.Conn) {
	defer client.Close()
	chDone := make(chan bool, 1)
	log.Debug("starting remote forward tunnel transfer ")

	// Start remote -> local data transfer
	go func() {
		_, err := io.Copy(client, local)
		if err != nil {
			log.Infof("error while copying %d->%d: %s", r.remotePort, r.localPort, err)
		}

		chDone <- true
	}()

	// Start local -> remote data transfer
	go func() {
		_, err := io.Copy(local, client)
		if err != nil {
			log.Infof("error while copying %d->%d: %s", r.localPort, r.remotePort, err)
		}
		chDone <- true
	}()

	log.Infof("started remote forward tunnel %d->%d successfully", r.remotePort, r.localPort)
	<-chDone
}
