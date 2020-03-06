package ssh

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
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
	w.Write([]byte(t.message))
}

func (t *testSSHHandler) listenAndServe(port int) error {
	ssh.Handle(func(s ssh.Session) {
		cmd := exec.Command("ssh-add", "-l")
		if ssh.AgentRequested(s) {
			l, err := ssh.NewAgentListener()
			if err != nil {
				log.Fatal(err)
			}
			defer l.Close()
			go ssh.ForwardAgentConnections(l, s)
			cmd.Env = append(s.Environ(), fmt.Sprintf("%s=%s", "SSH_AUTH_SOCK", l.Addr().String()))
		} else {
			cmd.Env = s.Environ()
		}
		cmd.Stdout = s
		cmd.Stderr = s.Stderr()
		if err := cmd.Run(); err != nil {
			log.Println(err)
			return
		}
	})

	return ssh.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

func TestForward(t *testing.T) {
	ctx := context.Background()
	ssh := testSSHHandler{}
	go ssh.listenAndServe(2222)
	fm := NewForwardManager(ctx, "localhost:2222", "localhost", "0.0.0.0")

	if err := connectForwards(fm); err != nil {
		t.Fatal(err)
	}

	if err := fm.Start("", ""); err != nil {
		t.Fatal(err)
	}

	if err := checkForwardsConnected(fm); err != nil {
		t.Fatal(err)
	}

	if err := callForwards(fm); err != nil {
		t.Error(err)
	}
}

func TestReverse(t *testing.T) {
	ctx := context.Background()
	fm := NewForwardManager(ctx, "localhost:2222", "localhost", "0.0.0.0")

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

func connectForwards(fm *ForwardManager) error {
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
			http.ListenAndServe(fmt.Sprintf(":%d", remote), handler)
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

		if err := fm.AddReverse(&model.Reverse{Local: local, Remote: remote}); err != nil {
			return err
		}

		go func() {
			handler := &testHTTPHandler{message: fmt.Sprintf("%d", local)}
			http.ListenAndServe(fmt.Sprintf(":%d", local), handler)
		}()
	}

	return nil
}

func checkForwardsConnected(fm *ForwardManager) error {
	tk := time.NewTicker(10 * time.Millisecond)
	connected := true
	for i := 0; i < 100; i++ {
		connected = true
		for _, f := range fm.forwards {
			connected = connected && f.connected
		}

		if connected {
			break
		}
		<-tk.C
	}

	if !connected {
		return fmt.Errorf("forwards not connected")
	}

	return nil
}

func checkReverseForwardsConnected(fm *ForwardManager) error {
	tk := time.NewTicker(10 * time.Millisecond)
	connected := true
	for i := 0; i < 100; i++ {
		connected = true
		for _, r := range fm.reverses {
			connected = connected && r.connected
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
