// Copyright 2022 The Okteto Authors
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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/ingressesv1"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (up *upContext) activate() error {

	oktetoLog.Infof("activating development container retry=%t", up.isRetry)

	if err := config.UpdateStateFile(up.Dev, config.Activating); err != nil {
		return err
	}

	// create a new context on every iteration
	ctx, cancel := context.WithCancel(context.Background())
	up.Cancel = cancel
	up.ShutdownCompleted = make(chan bool, 1)
	up.Sy = nil
	up.Forwarder = nil
	defer up.shutdown()

	up.Disconnect = make(chan error, 1)
	up.CommandResult = make(chan error, 1)
	up.cleaned = make(chan string, 1)
	up.hardTerminate = make(chan error, 1)

	app, create, err := utils.GetApp(ctx, up.Dev, up.Client, up.isRetry)
	if err != nil {
		return err
	}

	if v, ok := app.ObjectMeta().Annotations[model.OktetoAutoCreateAnnotation]; up.Dev.Autocreate && (!ok || v != model.OktetoUpCmd) {
		return oktetoErrors.UserError{
			E:    fmt.Errorf("app '%s' already exist", up.Dev.Name),
			Hint: "use a different name in your okteto.yaml, or remove the autocreate property",
		}
	}

	if up.isRetry && !apps.IsDevModeOn(app) {
		oktetoLog.Information("Development container has been deactivated")
		return nil
	}

	if err := app.RestoreOriginal(); err != nil {
		return err
	}

	if err := apps.ValidateMountPaths(app.PodSpec(), up.Dev); err != nil {
		return err
	}

	if _, err := registry.GetImageTagWithDigest(up.Dev.Image.Name); err == oktetoErrors.ErrNotFound {
		oktetoLog.Infof("image '%s' not found, building it: %s", up.Dev.Image.Name, err.Error())
		up.Options.Build = true
	}

	if !up.isRetry && up.Options.Build {
		if err := up.buildDevImage(ctx, app); err != nil {
			return fmt.Errorf("error building dev image: %s", err)
		}
	}

	go up.initializeSyncthing()

	if err := up.setDevContainer(app); err != nil {
		return err
	}

	if err := up.devMode(ctx, app, create); err != nil {
		if oktetoErrors.IsTransient(err) {
			return err
		}
		if strings.Contains(err.Error(), "Privileged containers are not allowed") && up.Dev.Docker.Enabled {
			return fmt.Errorf("docker support requires privileged containers. Privileged containers are not allowed in your current cluster")
		}
		if _, ok := err.(oktetoErrors.UserError); ok {
			return err
		}
		return fmt.Errorf("couldn't activate your development container\n    %s", err.Error())
	}

	if up.isRetry {
		analytics.TrackReconnect(true)
	}

	up.isRetry = true

	if err := up.forwards(ctx); err != nil {
		if err == oktetoErrors.ErrSSHConnectError {
			err := up.checkOktetoStartError(ctx, "Failed to connect to your development container")
			if err == oktetoErrors.ErrLostSyncthing {
				if err := pods.Destroy(ctx, up.Pod.Name, up.Dev.Namespace, up.Client); err != nil {
					return fmt.Errorf("error recreating development container: %s", err.Error())
				}
			}
			return err
		}
		return fmt.Errorf("couldn't connect to your development container: %s", err.Error())
	}
	go up.cleanCommand(ctx)

	if err := up.sync(ctx); err != nil {
		if up.shouldRetry(ctx, err) {
			return oktetoErrors.ErrLostSyncthing
		}
		return err
	}

	up.success = true

	go func() {
		output := <-up.cleaned
		oktetoLog.Debugf("clean command output: %s", output)

		outByCommand := strings.Split(strings.TrimSpace(output), "\n")
		var version, watches, hook string
		if len(outByCommand) >= 3 {
			version, watches, hook = outByCommand[0], outByCommand[1], outByCommand[2]

			if isWatchesConfigurationTooLow(watches) {
				folder := config.GetNamespaceHome(up.Dev.Namespace)
				if utils.GetWarningState(folder, ".remotewatcher") == "" {
					oktetoLog.Yellow("The value of /proc/sys/fs/inotify/max_user_watches in your cluster nodes is too low.")
					oktetoLog.Yellow("This can affect file synchronization performance.")
					oktetoLog.Yellow("Visit https://okteto.com/docs/reference/known-issues/ for more information.")
					if err := utils.SetWarningState(folder, ".remotewatcher", "true"); err != nil {
						oktetoLog.Infof("failed to set warning remotewatcher state: %s", err.Error())
					}
				}
			}

			if version != model.OktetoBinImageTag {
				oktetoLog.Yellow("The Okteto CLI version %s uses the init container image %s.", config.VersionString, model.OktetoBinImageTag)
				oktetoLog.Yellow("Please consider upgrading your init container image %s with the content of %s", up.Dev.InitContainer.Image, model.OktetoBinImageTag)
				oktetoLog.Infof("Using init image %s instead of default init image (%s)", up.Dev.InitContainer.Image, model.OktetoBinImageTag)
			}

		}
		divertURL := ""
		if up.Dev.Divert != nil {
			username := okteto.GetSanitizedUsername()
			name := model.DivertName(up.Dev.Divert.Ingress, username)
			i, err := ingressesv1.Get(ctx, name, up.Dev.Namespace, up.Client)
			if err != nil {
				oktetoLog.Errorf("error getting diverted ingress %s: %s", name, err.Error())
			} else if len(i.Spec.Rules) > 0 {
				divertURL = i.Spec.Rules[0].Host
			}
		}
		printDisplayContext(up.Dev, divertURL)
		durationActivateUp := time.Since(up.StartTime)
		analytics.TrackDurationActivateUp(durationActivateUp)
		if hook == "yes" {
			oktetoLog.Information("Running start.sh hook...")
			if err := up.runCommand(ctx, []string{"/var/okteto/cloudbin/start.sh"}); err != nil {
				up.CommandResult <- err
				return
			}
		}
		up.CommandResult <- up.runCommand(ctx, up.Dev.Command.Values)
	}()

	prevError := up.waitUntilExitOrInterruptOrApply(ctx)

	if up.shouldRetry(ctx, prevError) {
		if !up.Dev.PersistentVolumeEnabled() {
			if err := pods.Destroy(ctx, up.Pod.Name, up.Dev.Namespace, up.Client); err != nil {
				return err
			}
		}
		return oktetoErrors.ErrLostSyncthing
	}

	return prevError
}

func (up *upContext) shouldRetry(ctx context.Context, err error) bool {
	switch err {
	case nil:
		return false
	case oktetoErrors.ErrLostSyncthing:
		return true
	case oktetoErrors.ErrCommandFailed:
		return !up.Sy.Ping(ctx, false)
	case oktetoErrors.ErrApplyToApp:
		return true
	}

	return false
}

func (up *upContext) devMode(ctx context.Context, app apps.App, create bool) error {
	if err := up.createDevContainer(ctx, app, create); err != nil {
		return err
	}
	return up.waitUntilDevelopmentContainerIsRunning(ctx, app)
}

func (up *upContext) createDevContainer(ctx context.Context, app apps.App, create bool) error {
	spinner := utils.NewSpinner("Activating your development container...")
	spinner.Start()
	up.spinner = spinner
	defer spinner.Stop()

	if err := config.UpdateStateFile(up.Dev, config.Starting); err != nil {
		return err
	}

	if up.Dev.PersistentVolumeEnabled() {
		if err := volumes.CreateForDev(ctx, up.Dev, up.Client, up.Options.DevPath); err != nil {
			return err
		}
	}

	resetOnDevContainerStart := up.resetSyncthing || !up.Dev.PersistentVolumeEnabled()
	trMap, err := apps.GetTranslations(ctx, up.Dev, app, resetOnDevContainerStart, up.Client)
	if err != nil {
		return err
	}
	up.Translations = trMap

	if err := apps.TranslateDevMode(trMap); err != nil {
		return err
	}

	initSyncErr := <-up.hardTerminate
	if initSyncErr != nil {
		return initSyncErr
	}

	oktetoLog.Info("create deployment secrets")
	if err := secrets.Create(ctx, up.Dev, up.Client, up.Sy); err != nil {
		return err
	}

	var devApp apps.App
	for _, tr := range trMap {
		delete(tr.DevApp.ObjectMeta().Annotations, model.DeploymentRevisionAnnotation)
		if err := tr.DevApp.Deploy(ctx, up.Client); err != nil {
			return err
		}
		if err := tr.App.Deploy(ctx, up.Client); err != nil {
			return err
		}
		if tr.MainDev == tr.Dev {
			devApp = tr.DevApp
		}
	}

	if create {
		if err := services.CreateDev(ctx, up.Dev, up.Client); err != nil {
			return err
		}
	}

	pod, err := apps.GetRunningPodInLoop(ctx, up.Dev, devApp, up.Client)
	if err != nil {
		return err
	}

	up.Pod = pod

	return nil
}

func (up *upContext) waitUntilDevelopmentContainerIsRunning(ctx context.Context, app apps.App) error {
	msg := "Pulling images..."
	if up.Dev.PersistentVolumeEnabled() {
		msg = "Attaching persistent volume..."
		if err := config.UpdateStateFile(up.Dev, config.Attaching); err != nil {
			oktetoLog.Infof("error updating state: %s", err.Error())
		}
	}

	spinner := utils.NewSpinner(msg)
	spinner.Start()
	up.spinner = spinner
	defer spinner.Stop()

	optsWatchPod := metav1.ListOptions{
		Watch:         true,
		FieldSelector: fmt.Sprintf("metadata.name=%s", up.Pod.Name),
	}

	watcherPod, err := up.Client.CoreV1().Pods(up.Dev.Namespace).Watch(ctx, optsWatchPod)
	if err != nil {
		return err
	}

	optsWatchEvents := metav1.ListOptions{
		Watch:         true,
		FieldSelector: fmt.Sprintf("involvedObject.kind=Pod,involvedObject.name=%s", up.Pod.Name),
	}

	watcherEvents, err := up.Client.CoreV1().Events(up.Dev.Namespace).Watch(ctx, optsWatchEvents)
	if err != nil {
		return err
	}

	killing := false
	to := time.Now().Add(up.Dev.Timeout.Resources)
	var insufficientResourcesErr error
	for {
		if time.Now().After(to) && insufficientResourcesErr != nil {
			return oktetoErrors.UserError{E: fmt.Errorf("insufficient resources"),
				Hint: "Increase cluster resources or timeout of resources. More information is available here: https://okteto.com/docs/reference/manifest/#timeout-time-optional"}
		}
		select {
		case event := <-watcherEvents.ResultChan():
			if killing {
				continue
			}
			e, ok := event.Object.(*apiv1.Event)
			if !ok {
				oktetoLog.Infof("failed to cast event")
				watcherEvents, err = up.Client.CoreV1().Events(up.Dev.Namespace).Watch(ctx, optsWatchEvents)
				if err != nil {
					oktetoLog.Infof("error watching events: %s", err.Error())
					return err
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			podEvent, ok := event.Object.(*apiv1.Event)
			if !ok {
				continue
			}
			if up.Pod.UID != podEvent.InvolvedObject.UID {
				continue
			}
			optsWatchEvents.ResourceVersion = e.ResourceVersion
			oktetoLog.Infof("pod event: %s:%s", e.Reason, e.Message)
			switch e.Reason {
			case "Failed", "FailedScheduling", "FailedCreatePodSandBox", "ErrImageNeverPull", "InspectFailed", "FailedCreatePodContainer":
				if strings.Contains(e.Message, "pod has unbound immediate PersistentVolumeClaims") {
					continue
				}
				if strings.Contains(e.Message, "is in the cache, so can't be assumed") {
					continue
				}
				if strings.Contains(e.Message, "Insufficient cpu") || strings.Contains(e.Message, "Insufficient memory") {
					insufficientResourcesErr = fmt.Errorf(e.Message)
					spinner.Update("Insufficient cpu/memory in the cluster. Waiting for new nodes to come up...")
					continue
				}
				return fmt.Errorf(e.Message)
			case "SuccessfulAttachVolume":
				spinner.Stop()
				oktetoLog.Success("Persistent volume successfully attached")
				spinner.Update("Pulling images...")
				spinner.Start()
			case "Killing":
				if app.Kind() == model.StatefulSet {
					killing = true
					continue
				}
				return oktetoErrors.ErrDevPodDeleted
			case "Started":
				if e.Message == "Started container okteto-init-data" {
					spinner.Update("Initializing persistent volume content...")
				}
			case "Pulling":
				message := getPullingMessage(e.Message, up.Dev.Namespace)
				spinner.Update(fmt.Sprintf("%s...", message))
				if err := config.UpdateStateFile(up.Dev, config.Pulling); err != nil {
					oktetoLog.Infof("error updating state: %s", err.Error())
				}
			}
		case event := <-watcherPod.ResultChan():
			pod, ok := event.Object.(*apiv1.Pod)
			if !ok {
				oktetoLog.Infof("failed to cast pod event")
				watcherPod, err = up.Client.CoreV1().Pods(up.Dev.Namespace).Watch(ctx, optsWatchPod)
				if err != nil {
					oktetoLog.Infof("error watching pod events: %s", err.Error())
					return err
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if pod.UID != up.Pod.UID {
				continue
			}

			oktetoLog.Infof("dev pod %s is now %s", pod.Name, pod.Status.Phase)
			if pod.Status.Phase == apiv1.PodRunning {
				spinner.Stop()
				oktetoLog.Success("Images successfully pulled")
				return nil
			}
			if pod.DeletionTimestamp != nil {
				return oktetoErrors.ErrDevPodDeleted
			}
		case <-ctx.Done():
			oktetoLog.Debug("call to waitUntilDevelopmentContainerIsRunning cancelled")
			return ctx.Err()
		}
	}
}

func getPullingMessage(message, namespace string) string {
	registry := okteto.Context().Registry
	if registry == "" {
		return message
	}
	toReplace := fmt.Sprintf("%s/%s", registry, namespace)
	return strings.Replace(message, toReplace, okteto.DevRegistry, 1)
}
