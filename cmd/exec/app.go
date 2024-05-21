// Copyright 2024 The Okteto Authors
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

package exec

import (
	"context"
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/cmd/status"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/kubernetes"
)

const (
	// maxRetriesUntilAppIsInDevMode is the maximum number of retries until the app is in dev mode
	maxRetriesUntilAppIsInDevMode = 10

	// retryIntervalUntilAppIsInDevMode is the interval between retries until the app is in dev mode
	retryIntervalUntilAppIsInDevMode = 500 * time.Millisecond // 0.5 seconds
)

type appRetriever struct {
	ioControl   *io.Controller
	k8sProvider okteto.K8sClientProvider

	newAutocreateAppGetter func(c kubernetes.Interface) *autocreateAppGetter
	newRunningAppGetter    func(c kubernetes.Interface) *runningAppGetter
}

// newAppRetriever creates a new appRetriever
func newAppRetriever(ioControl *io.Controller, k8sProvider okteto.K8sClientProvider) *appRetriever {
	return &appRetriever{
		ioControl:   ioControl,
		k8sProvider: k8sProvider,

		newAutocreateAppGetter: newAutocreateAppGetter,
		newRunningAppGetter:    newRunningAppGetter,
	}
}

// getContainer retrieves the container for the dev environment
func (ar *appRetriever) getApp(ctx context.Context, dev *model.Dev) (apps.App, error) {
	ar.ioControl.Logger().Info("start to retrieve app")
	c, _, err := ar.k8sProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s client: %w", err)
	}
	var devApp apps.App
	if dev.Autocreate {
		ar.ioControl.Logger().Debug("retrieving autocreated app")
		devApp, err = ar.newAutocreateAppGetter(c).GetApp(ctx, dev)
		if err != nil {
			return nil, fmt.Errorf("failed to get app: %w", err)
		}
	} else {
		ar.ioControl.Logger().Debug("retrieving app")
		app, err := ar.newRunningAppGetter(c).GetApp(ctx, dev)
		if err != nil {
			return nil, fmt.Errorf("failed to get app: %w", err)
		}
		devApp = app.DevClone()
	}

	if err := devApp.Refresh(ctx, c); err != nil {
		return nil, fmt.Errorf("failed to refresh app: %w", err)
	}

	ar.ioControl.Logger().Debug("finish to retrieve app")
	return devApp, nil
}

type autocreateAppGetter struct {
	k8sClient kubernetes.Interface
}

func newAutocreateAppGetter(c kubernetes.Interface) *autocreateAppGetter {
	return &autocreateAppGetter{
		k8sClient: c,
	}
}

func (a *autocreateAppGetter) GetApp(ctx context.Context, dev *model.Dev) (apps.App, error) {
	dev.Name = model.DevCloneName(dev.Name)
	app, err := apps.Get(ctx, dev, dev.Namespace, a.k8sClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}
	return app, nil
}

type runningAppGetter struct {
	k8sClient kubernetes.Interface

	waiter []waiter
}

type waiter interface {
	Wait(dev *model.Dev, app apps.App) error
}

func newRunningAppGetter(c kubernetes.Interface) *runningAppGetter {
	return &runningAppGetter{
		k8sClient: c,
		waiter: []waiter{
			waitUntilAppIsInDevMode{c},
			waitUnitlDevModeIsReady{
				statusWaiter: status.Wait,
			},
		},
	}
}

func (r *runningAppGetter) GetApp(ctx context.Context, dev *model.Dev) (apps.App, error) {
	app, err := apps.Get(ctx, dev, dev.Namespace, r.k8sClient)
	if err != nil {
		return nil, err
	}
	for _, w := range r.waiter {
		if err := w.Wait(dev, app); err != nil {
			return nil, err
		}
	}
	return app, nil
}

type waitUntilAppIsInDevMode struct {
	k8sClient kubernetes.Interface
}

func (w waitUntilAppIsInDevMode) Wait(dev *model.Dev, app apps.App) error {
	ticker := time.NewTicker(retryIntervalUntilAppIsInDevMode)
	defer ticker.Stop()

	for i := 0; i < maxRetriesUntilAppIsInDevMode; i++ {
		if apps.IsDevModeOn(app) {
			return nil
		}
		<-ticker.C
	}
	return oktetoErrors.UserError{
		E:    fmt.Errorf("development mode is not enabled"),
		Hint: "Run 'okteto up' to enable it and try again",
	}
}

type waitUnitlDevModeIsReady struct {
	statusWaiter func(dev *model.Dev, states []config.UpState) error
}

func (w waitUnitlDevModeIsReady) Wait(dev *model.Dev, app apps.App) error {
	waitForStates := []config.UpState{config.Ready}
	if err := w.statusWaiter(dev, waitForStates); err != nil {
		return fmt.Errorf("failed to wait for dev mode to be ready: %w", err)
	}
	return nil
}
