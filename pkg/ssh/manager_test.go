// Copyright 2021 The Okteto Authors
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
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

type testHTTPHandler struct {
	message string
}
type testSSHHandler struct{}

func (t *testHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(fmt.Sprintf("message %s", t.message))
	_, _ = w.Write([]byte(t.message))
}

func (t *testSSHHandler) listenAndServe(address string) {
	forwardHandler := &ssh.ForwardedTCPHandler{}
	server := &ssh.Server{
		Addr: address,
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"direct-tcpip": ssh.DirectTCPIPHandler,
			"session":      ssh.DefaultSessionHandler,
		},
		LocalPortForwardingCallback: ssh.LocalPortForwardingCallback(func(ctx ssh.Context, dhost string, dport uint32) bool {
			log.Println("Accepted forward", dhost, dport)
			return true
		}),
		ReversePortForwardingCallback: ssh.ReversePortForwardingCallback(func(ctx ssh.Context, host string, port uint32) bool {
			log.Println("attempt to bind", host, port, "granted")
			return true
		}),
		RequestHandlers: map[string]ssh.RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		},
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf(err.Error())
	}
}

func TestForward(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sshPort, err := model.GetAvailablePort(model.Localhost)
	if err != nil {
		t.Fatal(err)
	}

	sshAddr := fmt.Sprintf("localhost:%d", sshPort)
	ssh := testSSHHandler{}
	go ssh.listenAndServe(sshAddr)
	fm := NewForwardManager(ctx, sshAddr, model.Localhost, "0.0.0.0", nil, "")

	if err := startServers(fm); err != nil {
		t.Fatal(err)
	}

	if err := fm.Start("", ""); err != nil {
		t.Fatal(err)
	}

	if err := fm.waitForwardsConnected(); err != nil {
		t.Fatal(err)
	}

	log.Info("forwards connected")

	if err := callForwards(fm); err != nil {
		t.Error(err)
	}

	cancel()
	fm.Stop()
	if err := fm.waitForwardsDisconnected(); err != nil {
		t.Error(err)
	}

	if !fm.pool.stopped {
		t.Error("pool is not stopped")
	}
}

func TestReverse(t *testing.T) {
	ctx := context.Background()
	sshPort, err := model.GetAvailablePort(model.Localhost)
	if err != nil {
		t.Fatal(err)
	}

	sshAddr := fmt.Sprintf("localhost:%d", sshPort)
	ssh := testSSHHandler{}
	go ssh.listenAndServe(sshAddr)
	fm := NewForwardManager(ctx, sshAddr, model.Localhost, "0.0.0.0", nil, "")

	if err := connectReverseForwards(fm); err != nil {
		t.Fatal(err)
	}

	if err := fm.Start("", ""); err != nil {
		t.Fatal(err)
	}

	if err := checkReverseForwardsConnected(fm); err != nil {
		t.Fatal(err)
	}

	if err := callReverseForwards(fm); err != nil {
		t.Error(err)
	}

}

func startServers(fm *ForwardManager) error {
	for i := 0; i < 1; i++ {
		local, err := model.GetAvailablePort(model.Localhost)
		if err != nil {
			return err
		}

		remote, err := model.GetAvailablePort(model.Localhost)
		if err != nil {
			return err
		}

		if err := fm.Add(model.Forward{Local: local, Remote: remote}); err != nil {
			return err
		}

		go func() {
			handler := &testHTTPHandler{message: fmt.Sprintf("%d", remote)}
			_ = http.ListenAndServe(fmt.Sprintf(":%d", remote), handler)
		}()
	}

	return nil
}

func connectReverseForwards(fm *ForwardManager) error {
	for i := 0; i < 1; i++ {
		local, err := model.GetAvailablePort(model.Localhost)
		if err != nil {
			return err
		}

		remote, err := model.GetAvailablePort(model.Localhost)
		if err != nil {
			return err
		}

		if err := fm.AddReverse(model.Reverse{Local: local, Remote: remote}); err != nil {
			return err
		}

		go func() {
			handler := &testHTTPHandler{message: fmt.Sprintf("%d", local)}
			_ = http.ListenAndServe(fmt.Sprintf(":%d", local), handler)
		}()
	}

	return nil
}

func checkReverseForwardsConnected(fm *ForwardManager) error {
	tk := time.NewTicker(10 * time.Millisecond)
	var connected bool
	for i := 0; i < 100; i++ {
		connected = true
		for _, r := range fm.reverses {
			connected = connected && r.connected()
		}

		if connected {
			break
		}
		<-tk.C
	}

	if !connected {
		return fmt.Errorf("reverse forwards not connected")
	}

	return nil
}

func callForwards(fm *ForwardManager) error {
	for _, f := range fm.forwards {
		localPort := getPort(f.localAddress)
		r, err := http.Get(fmt.Sprintf("http://localhost:%s", localPort))
		if err != nil {
			return fmt.Errorf("%s failed: %w", f.String(), err)
		}

		if r.StatusCode != 200 {
			return fmt.Errorf("%s bad response: %d | %s ", f.String(), r.StatusCode, r.Status)
		}

		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("%s failed to read response: %w", f.String(), err)
		}

		got := string(body)
		remotePort := getPort(f.remoteAddress)
		if got != remotePort {
			return fmt.Errorf("%s got: %s, expected: %s", f.String(), got, remotePort)
		}
	}

	return nil
}

func callReverseForwards(fm *ForwardManager) error {
	for _, r := range fm.reverses {
		remotePort := getPort(r.remoteAddress)
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s", remotePort))
		if err != nil {
			return fmt.Errorf("%s failed: %w", r.String(), err)
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("%s bad response: %d | %s ", r.String(), resp.StatusCode, resp.Status)
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("%s failed to read response: %w", r.String(), err)
		}

		got := string(body)
		localPort := getPort(r.localAddress)
		expected := localPort
		if got != expected {
			return fmt.Errorf("%s got: %s, expected: %s", r.String(), got, expected)
		}
	}

	return nil
}

func getPort(address string) string {
	parts := strings.Split(address, ":")
	return parts[1]
}

func (fm *ForwardManager) waitForwardsConnected() error {
	connectTimeout := 120 * time.Second
	tk := time.NewTicker(500 * time.Millisecond)
	start := time.Now()
	var connected bool

	for {
		elapsed := time.Since(start)
		if elapsed > connectTimeout {
			return fmt.Errorf("forwards not connected after %s", connectTimeout)
		}

		connected = true
		for _, f := range fm.forwards {
			connected = connected && f.connected()
		}

		if connected {
			return nil
		}
		<-tk.C
	}
}

func (fm *ForwardManager) waitForwardsDisconnected() error {
	connectTimeout := 120 * time.Second
	tk := time.NewTicker(500 * time.Millisecond)
	start := time.Now()

	for {
		elapsed := time.Since(start)
		if elapsed > connectTimeout {
			return fmt.Errorf("forwards not disconnected after %s", connectTimeout)
		}

		disconnected := true
		for _, f := range fm.forwards {
			if f.connected() {
				log.Infof("%s is still connected", f)
				disconnected = false
			}
		}

		if disconnected {
			return nil
		}

		<-tk.C
	}
}

func TestAdd(t *testing.T) {

	pf := NewForwardManager(context.Background(), "0.0.0.0:22000", "0.0.0.0", "0.0.0.0", nil, "")
	if err := pf.Add(model.Forward{Local: 10010, Remote: 1010}); err != nil {
		t.Fatal(err)
	}

	if err := pf.Add(model.Forward{Local: 10011, Remote: 1011}); err != nil {
		t.Fatal(err)
	}

	if err := pf.Add(model.Forward{Local: 10010, Remote: 1011}); err == nil {
		t.Fatal("duplicated local port didn't return an error")
	}

	if err := pf.Add(model.Forward{Local: 10012, Remote: 15123, Service: true, ServiceName: "svc"}); err != nil {
		t.Fatal(err)
	}

	if pf.forwards[10012].remoteAddress != "svc:15123" {
		t.Fatalf("expected 'svc:15123', got '%s'", pf.forwards[1012].remoteAddress)
	}
}
