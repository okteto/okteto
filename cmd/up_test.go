package cmd

import (
	"fmt"
	"testing"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
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

func Test_printDisplayContext(t *testing.T) {
	var tests = []struct {
		name string
		dev  *model.Dev
	}{
		{
			name: "basic",
			dev: &model.Dev{
				Name:      "dev",
				Namespace: "namespace",
			},
		},
		{
			name: "single-forward",
			dev: &model.Dev{
				Name:      "dev",
				Namespace: "namespace",
				Forward:   []model.Forward{{Local: 1000, Remote: 1000}},
			},
		},
		{
			name: "multiple-forward",
			dev: &model.Dev{
				Name:      "dev",
				Namespace: "namespace",
				Forward:   []model.Forward{{Local: 1000, Remote: 1000}, {Local: 2000, Remote: 2000}},
			},
		},
		{
			name: "single-reverse",
			dev: &model.Dev{
				Name:      "dev",
				Namespace: "namespace",
				Reverse:   []model.Reverse{{Local: 1000, Remote: 1000}},
			},
		},
		{
			name: "multiple-reverse",
			dev: &model.Dev{
				Name:      "dev",
				Namespace: "namespace",
				Reverse:   []model.Reverse{{Local: 1000, Remote: 1000}, {Local: 2000, Remote: 2000}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			printDisplayContext(tt.name, tt.dev)
		})
	}

}
