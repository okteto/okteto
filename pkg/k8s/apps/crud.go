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

package apps

import (
	"context"
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type ErrApplicationNotFound struct {
	Name string
}

func (e ErrApplicationNotFound) Error() string {
	return fmt.Sprintf("the application '%s' referred by your okteto manifest doesn't exist", e.Name)
}
func Get(ctx context.Context, dev *model.Dev, namespace string, c kubernetes.Interface) (App, error) {
	d, err := deployments.GetByDev(ctx, dev, namespace, c)

	if err == nil {
		return &DeploymentApp{d: d}, nil
	}

	if !oktetoErrors.IsNotFound(err) {
		return nil, err
	}

	sfs, err := statefulsets.GetByDev(ctx, dev, namespace, c)
	if err != nil {
		if oktetoErrors.IsNotFound(err) {
			return nil, ErrApplicationNotFound{Name: dev.Name}
		}
		return nil, err
	}
	return &StatefulSetApp{sfs: sfs}, nil
}

// IsDevModeOn returns if a statefulset is in devmode
func IsDevModeOn(app App) bool {
	return app.ObjectMeta().Labels[constants.DevLabel] == "true" || len(app.ObjectMeta().Labels[model.DevCloneLabel]) > 0
}

// SetLastBuiltAnnotation sets the app timestamp
func SetLastBuiltAnnotation(app App) {
	app.ObjectMeta().Annotations[model.LastBuiltAnnotation] = time.Now().UTC().Format(constants.TimeFormat)
}

// GetRunningPodInLoop returns the dev pod for an app and loops until it success
func GetRunningPodInLoop(ctx context.Context, dev *model.Dev, app App, c kubernetes.Interface) (*apiv1.Pod, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	start := time.Now()
	to := start.Add(dev.Timeout.Resources)

	for retries := 0; ; retries++ {
		err := app.Refresh(ctx, c)
		if err != nil {
			return nil, err
		}
		if err = app.CheckConditionErrors(dev); err != nil {
			return nil, err
		}

		pod, err := app.GetRunningPod(ctx, c)

		if err == nil {
			return pod, nil
		}

		if !oktetoErrors.IsNotFound(err) {
			return nil, err
		}

		if time.Now().After(to) && retries > 10 {
			return nil, oktetoErrors.ErrKubernetesLongTimeToCreateDevContainer
		}

		select {
		case <-ticker.C:
			if retries%5 == 0 {
				oktetoLog.Info("development container is not ready yet, will retry")
			}
			continue
		case <-ctx.Done():
			oktetoLog.Debug("call to apps.GetRunningPodInLoop cancelled")
			return nil, ctx.Err()
		}
	}
}

// GetTranslations fills all the deployments pointed by a development container
func GetTranslations(ctx context.Context, dev *model.Dev, app App, reset bool, c kubernetes.Interface) (map[string]*Translation, error) {
	mainTr := &Translation{
		MainDev: dev,
		Dev:     dev,
		App:     app,
		Rules:   []*model.TranslationRule{dev.ToTranslationRule(dev, reset)},
	}
	result := map[string]*Translation{app.ObjectMeta().Name: mainTr}

	if err := loadServiceTranslations(ctx, dev, reset, result, c); err != nil {
		return nil, err
	}

	for _, tr := range result {
		for _, rule := range tr.Rules {
			devContainer := GetDevContainer(tr.App.PodSpec(), rule.Container)
			if devContainer == nil {
				return nil, fmt.Errorf("%s '%s': container '%s' not found", tr.App.Kind(), tr.App.ObjectMeta().Name, rule.Container)
			}
			rule.Container = devContainer.Name
			if rule.Image == "" {
				rule.Image = devContainer.Image
			}
		}
	}

	return result, nil
}

func loadServiceTranslations(ctx context.Context, dev *model.Dev, reset bool, result map[string]*Translation, c kubernetes.Interface) error {
	for _, s := range dev.Services {
		app, err := Get(ctx, s, dev.Namespace, c)
		if err != nil {
			return err
		}

		rule := s.ToTranslationRule(dev, reset)

		if _, ok := result[app.ObjectMeta().Name]; ok {
			result[app.ObjectMeta().Name].Rules = append(result[app.ObjectMeta().Name].Rules, rule)
			continue
		}

		result[app.ObjectMeta().Name] = &Translation{
			MainDev: dev,
			Dev:     s,
			App:     app,
			Rules:   []*model.TranslationRule{rule},
		}
	}

	return nil
}

// TranslateDevMode translates the deployment manifests to put them in dev mode
func TranslateDevMode(trMap map[string]*Translation) error {
	for _, tr := range trMap {
		err := tr.translate()
		if err != nil {
			return err
		}
	}
	return nil
}

// ListDevModeOn returns a list of strings with the names of deployments or statefulsets in DevMode.
// If no app is found in dev mode, an empty slice is returned
func ListDevModeOn(ctx context.Context, manifest *model.Manifest, c kubernetes.Interface) []string {
	devModeApps := make([]string, 0)
	for name, dev := range manifest.Dev {
		// when autocreate is active, the app name has suffix -okteto
		// this should be taken into account when searching for dev mode apps
		// we just want to modify the dev
		var appDev = *dev

		if appDev.Autocreate {
			appDev.Name = model.DevCloneName(appDev.Name)
		}
		app, err := Get(ctx, &appDev, manifest.Namespace, c)
		if err != nil {
			oktetoLog.Debugf("error listing dev-mode %s: %v", name, err)
		}
		if app != nil && IsDevModeOn(app) {
			// only add to slice the dev apps
			devModeApps = append(devModeApps, name)
		}
	}
	return devModeApps
}
