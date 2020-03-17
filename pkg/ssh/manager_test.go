package ssh

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gliderlabs/ssh"
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
		log.Fatal(err)
	}
}

func TestForward(t *testing.T) {
	ctx := context.Background()
	sshPort, err := model.GetAvailablePort()
	if err != nil {
		t.Fatal(err)
	}

	sshAddr := fmt.Sprintf("localhost:%d", sshPort)
	ssh := testSSHHandler{}
	go ssh.listenAndServe(sshAddr)
	fm := NewForwardManager(ctx, sshAddr, "localhost", "0.0.0.0", nil)

	if err := startServers(fm); err != nil {
		t.Fatal(err)
	}

	if err := fm.Start("", ""); err != nil {
		t.Fatal(err)
	}

	if err := fm.waitForwardsConnected(); err != nil {
		t.Fatal(err)
	}

	log.Print("forwards connected")

	if err := callForwards(fm); err != nil {
		t.Error(err)
	}
}

func TestReverse(t *testing.T) {
	ctx := context.Background()
	sshPort, err := model.GetAvailablePort()
	if err != nil {
		t.Fatal(err)
	}

	sshAddr := fmt.Sprintf("localhost:%d", sshPort)
	ssh := testSSHHandler{}
	go ssh.listenAndServe(sshAddr)
	fm := NewForwardManager(ctx, sshAddr, "localhost", "0.0.0.0", nil)

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
		local, err := model.GetAvailablePort()
		if err != nil {
			return err
		}

		remote, err := model.GetAvailablePort()
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
		local, err := model.GetAvailablePort()
		if err != nil {
			return err
		}

		remote, err := model.GetAvailablePort()
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
