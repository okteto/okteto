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

package up

import (
	"fmt"
	"testing"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
)

func Test_waitUntilExitOrInterrupt(t *testing.T) {
	up := upContext{}
	up.CommandResult = make(chan error, 1)
	up.CommandResult <- nil
	err := up.waitUntilExitOrInterrupt()
	if err != nil {
		t.Errorf("exited with error instead of nil: %s", err)
	}

	up.CommandResult <- fmt.Errorf("custom-error")
	err = up.waitUntilExitOrInterrupt()
	if err == nil {
		t.Errorf("didn't report proper error")
	}
	if _, ok := err.(errors.CommandError); !ok {
		t.Errorf("didn't translate the error: %s", err)
	}

	up.Disconnect = make(chan error, 1)
	up.Disconnect <- errors.ErrLostSyncthing
	err = up.waitUntilExitOrInterrupt()
	if err != errors.ErrLostSyncthing {
		t.Errorf("exited with error %s instead of %s", err, errors.ErrLostSyncthing)
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
			printDisplayContext(tt.dev, "")
		})
	}

}
