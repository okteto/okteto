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

package up

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (up *upContext) activate() error {

	oktetoLog.Infof("activating development container retry=%t", up.isRetry)

	if err := config.UpdateStateFile(up.Dev.Name, up.Dev.Namespace, config.Activating); err != nil {
		return err
	}

	// create a new context on every iteration
	ctx, cancel := context.WithCancel(context.Background())
	up.Cancel = cancel
	up.ShutdownCompleted = make(chan bool, 1)
	up.Sy = nil
	up.Forwarder = nil
	defer func() {
		if !up.interruptReceived {
			up.shutdown()
		}
	}()

	up.Disconnect = make(chan error, 1)
	up.CommandResult = make(chan error, 1)
	up.cleaned = make(chan string, 1)
	up.hardTerminate = make(chan error, 1)

	k8sClient, _, err := up.K8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}
	app, create, err := utils.GetApp(ctx, up.Dev, k8sClient, up.isRetry)
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
		if up.Dev.IsHybridModeEnabled() {
			up.shutdownHybridMode()
		}
		oktetoLog.Information("Development container has been deactivated")
		return nil
	}

	if err := app.RestoreOriginal(); err != nil {
		return err
	}

	if err := apps.ValidateMountPaths(app.PodSpec(), up.Dev); err != nil {
		return err
	}

	buildDevImage := false
	if _, err := up.Registry.GetImageTagWithDigest(up.Dev.Image.Name); err == oktetoErrors.ErrNotFound {
		oktetoLog.Infof("image '%s' not found, building it: %s", up.Dev.Image.Name, err.Error())
		path := up.Dev.Image.GetDockerfilePath(up.Fs)
		if _, err := os.Stat(path); err != nil {
			return oktetoErrors.UserError{
				E:    fmt.Errorf("the image '%s' doesn't exist and Dockerfile '%s' is not accessible", up.Dev.Image.Name, path),
				Hint: "Please update your build section and try again",
			}
		}
		buildDevImage = true
	}

	if !up.isRetry && buildDevImage {
		if err := up.buildDevImage(ctx, app); err != nil {
			return fmt.Errorf("error building dev image: %w", err)
		}
	}

	go func() {
		if err := up.initializeSyncthing(); err != nil {
			oktetoLog.Infof("could not initialize syncthing: %s", err)
		}
	}()
	if err := up.setDevContainer(app); err != nil {
		return err
	}

	var lastPodUID types.UID
	if up.Pod != nil {
		// if up.Pod exists save the UID before devMode to compare when retry
		lastPodUID = up.Pod.UID
	}

	if err := up.waitUntilAppIsAwaken(ctx, app); err != nil {
		oktetoLog.Infof("error waiting for the original %s to be awaken: %s", app.Kind(), err.Error())
		return err
	}

	if err := up.devMode(ctx, app, create); err != nil {
		if oktetoErrors.IsTransient(err) {
			return err
		}
		if _, ok := err.(oktetoErrors.UserError); ok {
			return err
		}
		return fmt.Errorf("couldn't activate your development container\n    %w", err)
	}

	if up.isRetry {
		if lastPodUID != up.Pod.UID {
			up.analyticsMeta.ReconnectDevPodRecreated()
		} else {
			up.analyticsMeta.ReconnectDefault()
		}
	}

	up.isRetry = true

	if err := up.forwards(ctx); err != nil {
		if err == oktetoErrors.ErrSSHConnectError {
			err := up.checkOktetoStartError(ctx, "Failed to connect to your development container")
			if err == oktetoErrors.ErrLostSyncthing {
				if err := pods.Destroy(ctx, up.Pod.Name, up.Dev.Namespace, k8sClient); err != nil {
					return fmt.Errorf("error recreating development container: %w", err)
				}
			}
			return err
		}
		return fmt.Errorf("couldn't connect to your development container: %w", err)
	}
	go up.cleanCommand(ctx)

	if err := up.sync(ctx); err != nil {
		if up.shouldRetry(ctx, err) {
			return oktetoErrors.ErrLostSyncthing
		}
		return err
	}

	// success means all context is ready to run the activation
	up.success = true

	go func() {
		output := <-up.cleaned
		oktetoLog.Debugf("clean command output: %s", output)

		outByCommand := strings.Split(strings.TrimSpace(output), "\n")
		var version, watches string
		minOutCmdParts := 3
		if len(outByCommand) >= minOutCmdParts {
			version, watches = outByCommand[0], outByCommand[1]

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
		printDisplayContext(up)
		durationActivateUp := time.Since(up.StartTime)
		up.analyticsMeta.ActivateDuration(durationActivateUp)

		startRunCommand := time.Now()
		up.CommandResult <- up.RunCommand(ctx, up.Dev.Command.Values)
		up.analyticsMeta.ExecDuration(time.Since(startRunCommand))

	}()

	prevError := up.waitUntilExitOrInterruptOrApply(ctx)

	if up.shouldRetry(ctx, prevError) {
		if !up.Dev.PersistentVolumeEnabled() {
			if err := pods.Destroy(ctx, up.Pod.Name, up.Dev.Namespace, k8sClient); err != nil {
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
	startCreateDev := time.Now()
	if err := up.createDevContainer(ctx, app, create); err != nil {
		return err
	}
	up.analyticsMeta.DevContainerCreation(time.Since(startCreateDev))
	return up.waitUntilDevelopmentContainerIsRunning(ctx, app)
}

func (up *upContext) createDevContainer(ctx context.Context, app apps.App, create bool) error {
	msg := "Preparing development environment..."
	if !up.Dev.IsHybridModeEnabled() {
		msg = "Activating your development container..."
	}
	oktetoLog.Spinner(msg)
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	if err := config.UpdateStateFile(up.Dev.Name, up.Dev.Namespace, config.Starting); err != nil {
		return err
	}

	k8sClient, _, err := up.K8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}

	if up.Dev.PersistentVolumeEnabled() {
		if err := volumes.CreateForDev(ctx, up.Dev, k8sClient, up.Options.ManifestPath); err != nil {
			return err
		}
	}

	resetOnDevContainerStart := up.resetSyncthing || !up.Dev.PersistentVolumeEnabled()
	trMap, err := apps.GetTranslations(ctx, up.Dev, app, resetOnDevContainerStart, k8sClient)
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
	if err := secrets.Create(ctx, up.Dev, k8sClient, up.Sy); err != nil {
		return err
	}

	var devApp apps.App
	for _, tr := range trMap {
		delete(tr.DevApp.ObjectMeta().Annotations, model.DeploymentRevisionAnnotation)
		if err := tr.DevApp.Deploy(ctx, k8sClient); err != nil {
			return err
		}
		if err := tr.App.Deploy(ctx, k8sClient); err != nil {
			return err
		}
		if tr.MainDev == tr.Dev {
			devApp = tr.DevApp
		}
	}

	if create {
		if err := services.CreateDev(ctx, up.Dev, k8sClient); err != nil {
			return err
		}
	}

	pod, err := apps.GetRunningPodInLoop(ctx, up.Dev, devApp, k8sClient)
	if err != nil {
		return err
	}

	up.Pod = pod

	return nil
}

func (up *upContext) waitUntilDevelopmentContainerIsRunning(ctx context.Context, app apps.App) error {
	msg := "Preparing development environment..."
	if !up.Dev.IsHybridModeEnabled() {
		msg = "Pulling images..."
		if up.Dev.PersistentVolumeEnabled() {
			msg = "Attaching persistent volume..."
			if err := config.UpdateStateFile(up.Dev.Name, up.Dev.Namespace, config.Attaching); err != nil {
				oktetoLog.Infof("error updating state: %s", err.Error())
			}
		}
	}
	oktetoLog.Spinner(msg)
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	k8sClient, _, err := up.K8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}

	optsWatchPod := metav1.ListOptions{
		Watch:         true,
		FieldSelector: fmt.Sprintf("metadata.name=%s", up.Pod.Name),
	}

	watcherPod, err := k8sClient.CoreV1().Pods(up.Dev.Namespace).Watch(ctx, optsWatchPod)
	if err != nil {
		return err
	}

	optsWatchEvents := metav1.ListOptions{
		Watch:         true,
		FieldSelector: fmt.Sprintf("involvedObject.kind=Pod,involvedObject.name=%s", up.Pod.Name),
	}

	watcherEvents, err := k8sClient.CoreV1().Events(up.Dev.Namespace).Watch(ctx, optsWatchEvents)
	if err != nil {
		return err
	}

	killing := false
	to := time.Now().Add(up.Dev.Timeout.Resources)
	ticker := time.NewTicker(10 * time.Second)
	var failedSchedulingEvent *apiv1.Event
	for {
		if failedSchedulingEvent != nil && time.Now().After(to) {
			// this provides 2 min for "FailedScheduling" to resolve by themselves
			if strings.Contains(failedSchedulingEvent.Message, "Insufficient cpu") || strings.Contains(failedSchedulingEvent.Message, "Insufficient memory") {
				return oktetoErrors.UserError{E: fmt.Errorf("insufficient resources"),
					Hint: "Increase cluster resources or timeout of resources. More information is available here: https://okteto.com/docs/reference/manifest/#timeout-time-optional"}
			}
			return fmt.Errorf(failedSchedulingEvent.Message)
		}
		select {
		case <-ticker.C:
			oktetoLog.Info("ticker event in 'waitUntilDevelopmentContainerIsRunning' loop...")
			continue
		case event := <-watcherEvents.ResultChan():
			if killing {
				continue
			}
			e, ok := event.Object.(*apiv1.Event)
			if !ok {
				oktetoLog.Infof("failed to cast event")
				watcherEvents, err = k8sClient.CoreV1().Events(up.Dev.Namespace).Watch(ctx, optsWatchEvents)
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
			oktetoLog.Infof("pod event: %s:%s:%s", e.Reason, e.Type, e.Message)
			switch e.Reason {
			case "FailedScheduling":
				if failedSchedulingEvent == nil {
					failedSchedulingEvent = e
					to = time.Now().Add(up.Dev.Timeout.Resources)
				}
				continue
			case "TriggeredScaleUp":
				failedSchedulingEvent = nil
				oktetoLog.Spinner("Waiting for the cluster to scale up...")
				continue
			case "Failed", "FailedCreatePodSandBox", "ErrImageNeverPull", "InspectFailed", "FailedCreatePodContainer":
				if strings.Contains(e.Message, "pod has unbound immediate PersistentVolumeClaims") {
					continue
				}
				if strings.Contains(e.Message, "is in the cache, so can't be assumed") {
					continue
				}
				if strings.Contains(e.Message, "failed to create subPath directory") {
					return oktetoErrors.UserError{
						E: fmt.Errorf("there is no space left in persistent volume"),
						Hint: fmt.Sprintf(`Okteto volume is full.
    Increase your persistent volume size, run '%s' and try 'okteto up' again.
    More information about configuring your persistent volume at https://okteto.com/docs/reference/manifest/#persistentvolume-object-optional`, utils.GetDownCommand(up.Options.ManifestPathFlag)),
					}
				}
				if e.Type == "Warning" && strings.Contains(e.Message, "container veth name provided (eth0) already exists") {
					oktetoLog.Infof("pod event: %s:%s:%s", e.Reason, e.Type, e.Message)
					continue
				}
				return fmt.Errorf(e.Message)
			case "SuccessfulAttachVolume":
				failedSchedulingEvent = nil
				oktetoLog.Success("Persistent volume successfully attached")
				oktetoLog.Spinner("Pulling images...")
			case "Killing":
				if app.Kind() == okteto.StatefulSet {
					killing = true
					continue
				}
				return oktetoErrors.ErrDevPodDeleted
			case "Started":
				failedSchedulingEvent = nil
				if e.Message == "Started container okteto-init-data" {
					oktetoLog.Spinner("Initializing persistent volume content...")
				}
			case "Pulling":
				failedSchedulingEvent = nil
				message := getPullingMessage(e.Message, up.Dev.Namespace)
				oktetoLog.Spinner(fmt.Sprintf("%s...", message))
				if err := config.UpdateStateFile(up.Dev.Name, up.Dev.Namespace, config.Pulling); err != nil {
					oktetoLog.Infof("error updating state: %s", err.Error())
				}
			}
		case event := <-watcherPod.ResultChan():
			pod, ok := event.Object.(*apiv1.Pod)
			if !ok {
				oktetoLog.Infof("failed to cast pod event")
				watcherPod, err = k8sClient.CoreV1().Pods(up.Dev.Namespace).Watch(ctx, optsWatchPod)
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
				if !up.Dev.IsHybridModeEnabled() {
					oktetoLog.Success("Images successfully pulled")
				}
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
	registry := okteto.GetContext().Registry
	if registry == "" {
		return message
	}
	toReplace := fmt.Sprintf("%s/%s", registry, namespace)
	return strings.Replace(message, toReplace, constants.DevRegistry, 1)
}

// waitUntilAppIsAwaken waits until the app is awaken checking if the annotation dev.okteto.com/state-before-sleeping is present in the app resource
func (up *upContext) waitUntilAppIsAwaken(ctx context.Context, app apps.App) error {
	// If it is auto create, we don't need to wait for the app to wake up
	if up.Dev.Autocreate {
		return nil
	}

	k8sClient, _, err := up.K8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}

	appToCheck := app
	// If the app is already in dev mode, we need to check the cloned app to see if it is awaken
	if apps.IsDevModeOn(app) {
		var err error
		appToCheck, err = app.GetDevClone(ctx, k8sClient)
		if err != nil {
			return err
		}
	}

	if _, ok := appToCheck.ObjectMeta().Annotations[model.StateBeforeSleepingAnnontation]; !ok {
		return nil
	}

	timeout := 5 * time.Minute
	to := time.NewTicker(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer to.Stop()
	defer ticker.Stop()
	oktetoLog.Spinner(fmt.Sprintf("Dev environment '%s' is sleeping. Waiting for it to wake up...", appToCheck.ObjectMeta().Name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()
	for {
		select {
		case <-to.C:
			// In case of timeout, we just print a warning to avoid the command to fail
			oktetoLog.Warning("Dev environment '%s' didn't wake up after %s", appToCheck.ObjectMeta().Name, timeout.String())
			return nil
		case <-ticker.C:
			if err := appToCheck.Refresh(ctx, k8sClient); err != nil {
				return err
			}

			// If the app is not sleeping anymore, we are done
			if _, ok := appToCheck.ObjectMeta().Annotations[model.StateBeforeSleepingAnnontation]; !ok {
				return nil
			}
		}
	}
}
