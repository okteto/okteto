package cmd

import (
	"fmt"
	"testing"

	"github.com/okteto/okteto/pkg/errors"
)

func TestWaitUntilExitOrInterrupt(t *testing.T) {
	up := UpContext{}
	up.Running = make(chan error, 1)
	up.Running <- nil
	err := up.WaitUntilExitOrInterrupt()
	if err != nil {
		t.Errorf("exited with error instead of nil: %s", err)
	}

	up.Running <- fmt.Errorf("custom-error")
	err = up.WaitUntilExitOrInterrupt()
	if err == nil {
		t.Errorf("didn't report proper error")
	}

	if err != errors.ErrCommandFailed {
		t.Errorf("didn't translate the error: %s", err)
	}

	up.Disconnect = make(chan struct{}, 1)
	up.Disconnect <- struct{}{}
	err = up.WaitUntilExitOrInterrupt()
	if err != errors.ErrLostConnection {
		t.Errorf("exited with error %s instead of %s", err, errors.ErrLostConnection)
	}
}
