package ssh

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/model"
)

type testHTTPHandler struct {
	message string
}
type testSSHHandler struct{}

func (t *testHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(t.message))
}

/*func (t *testSSHHandler) ListenAndServe(port int) error {
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
}*/

func TestForwardManager(t *testing.T) {
	ctx := context.Background()
	fm := NewForwardManager(ctx, ":2222")
	if err := connectForwards(fm); err != nil {
		t.Fatal(err)
	}

	if err := connectReverseForwards(fm); err != nil {
		t.Fatal(err)
	}

	/*go func() {
		//s := &testSSHHandler{}
		//s.ListenAndServe(22000)
	}()*/

	if err := fm.Start(); err != nil {
		t.Fatal(err)
	}

	if err := checkForwardsConnected(fm); err != nil {
		t.Fatal(err)
	}

	if err := checkReverseForwardsConnected(fm); err != nil {
		t.Fatal(err)
	}

	if err := callForwards(fm); err != nil {
		t.Error(err)
	}

	if err := callReverseForwards(fm); err != nil {
		t.Error(err)
	}

}

func connectForwards(fm *ForwardManager) error {
	for i := 0; i < 3; i++ {
		local, err := model.GetAvailablePort()
		if err != nil {
			return err
		}

		remote, err := model.GetAvailablePort()
		if err != nil {
			return err
		}

		if err := fm.Add(&model.Forward{Local: local, Remote: remote}); err != nil {
			return err
		}

		go func() {
			http.ListenAndServe(fmt.Sprintf(":%d", remote), &testHTTPHandler{message: string(remote)})
		}()
	}

	return nil
}

func connectReverseForwards(fm *ForwardManager) error {
	for i := 0; i < 3; i++ {
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
			http.ListenAndServe(fmt.Sprintf(":%d", local), &testHTTPHandler{message: string(local)})
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
		r, err := http.Get(fmt.Sprintf("http://localhost:%d", f.localPort))
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
		if got != string(f.remotePort) {
			return fmt.Errorf("%s got: %s, expected: %d", f.String(), got, f.remotePort)
		}
	}

	return nil
}

func callReverseForwards(fm *ForwardManager) error {
	for _, r := range fm.reverses {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d", r.remotePort))
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
		expected := string(r.localPort)
		if got !=  expected {
			return fmt.Errorf("%s got: %s, expected: %s", r.String(), got, expected)
		}
	}

	return nil
}
