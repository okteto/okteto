// Copyright 2023 The Okteto Authors
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

package connect

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/k8s/apps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/spf13/afero"
	apiv1 "k8s.io/api/core/v1"
)

// forwarder manages port forwarding for the dev session.
type forwarder interface {
	Add(forward.Forward) error
	Start(string, string) error
	Stop()
}

// connectContext holds all state for an active connect session.
type connectContext struct {
	Namespace         string
	StartTime         time.Time
	analyticsTracker  analyticsTrackerInterface
	Fs                afero.Fs
	K8sClientProvider okteto.K8sClientProvider
	Forwarder         forwarder
	Disconnect        chan error
	Exit              chan error
	Sy                *syncthing.Syncthing
	hardTerminate     chan error
	ShutdownCompleted chan bool
	Translations      map[string]*apps.Translation
	Manifest          *model.Manifest
	analyticsMeta     *analytics.UpMetricsMetadata
	Dev               *model.Dev
	Options           *Options
	Pod               *apiv1.Pod
	Cancel            func()
	shutdownOnce      sync.Once
	isRetry           bool
	success           bool
}

// start runs the activation loop and blocks until the user stops the session.
// Ctrl+C stops local sync but leaves the dev container running.
func (c *connectContext) start() error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go c.activateLoop()

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		// Leave the dev container running — only stop local sync.
		c.shutdown()
		oktetoLog.Println()
	case err := <-c.Exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	return nil
}

// shutdown cancels the context and stops syncthing. The dev container is NOT torn down.
// It is safe to call concurrently or more than once per activation cycle — only the first
// call does work; subsequent calls return immediately.
func (c *connectContext) shutdown() {
	c.shutdownOnce.Do(func() {
		oktetoLog.StopSpinner()
		oktetoLog.Infof("starting shutdown sequence")

		if !c.success {
			c.analyticsMeta.FailActivate()
		}

		if c.Cancel != nil {
			c.Cancel()
			oktetoLog.Info("sent cancellation signal")
		}

		if c.Sy != nil {
			oktetoLog.Infof("stopping syncthing")
			if err := c.Sy.SoftTerminate(); err != nil {
				oktetoLog.Infof("failed to stop syncthing during shutdown: %s", err.Error())
			}
		}

		if c.Forwarder != nil {
			oktetoLog.Infof("stopping forwarders")
			c.Forwarder.Stop()
		}

		oktetoLog.Info("completed shutdown sequence")
		if c.ShutdownCompleted != nil {
			c.ShutdownCompleted <- true
		}
	})
}
