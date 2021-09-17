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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/diverts"
	"github.com/okteto/okteto/pkg/k8s/ingressesv1"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (up *upContext) activate(autoDeploy, build bool) error {
	log.Infof("activating development container retry=%t", up.isRetry)

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

	k8sObject, create, err := up.getCurrentK8sObject(ctx, autoDeploy)
	if err != nil {
		return err
	}

	if err := apps.ValidateMountPaths(k8sObject, up.Dev); err != nil {
		return err
	}

	if up.isRetry && !apps.IsDevModeOn(k8sObject) {
		log.Information("Development container has been deactivated")
		return nil
	}

	if apps.IsDevModeOn(k8sObject) && apps.HasBeenChanged(k8sObject) {
		return errors.UserError{
			E: fmt.Errorf("%s '%s' has been modified while your development container was active", strings.ToLower(string(k8sObject.ObjectType)), k8sObject.Name),
			Hint: `Follow these steps:
	  1. Execute 'okteto down'
	  2. Apply your manifest changes again: 'kubectl apply'
	  3. Execute 'okteto up' again
    More information is available here: https://okteto.com/docs/reference/known-issues/#kubectl-apply-changes-are-undone-by-okteto-up`,
		}
	}

	if _, err := registry.GetImageTagWithDigest(ctx, up.Dev.Namespace, up.Dev.Image.Name); err == errors.ErrNotFound {
		log.Infof("image '%s' not found, building it: %s", up.Dev.Image.Name, err.Error())
		build = true
	}

	if !up.isRetry && build {
		if err := up.buildDevImage(ctx, k8sObject, create); err != nil {
			return fmt.Errorf("error building dev image: %s", err)
		}
	}

	go up.initializeSyncthing()

	if err := up.setDevContainer(k8sObject); err != nil {
		return err
	}

	if err := up.devMode(ctx, k8sObject, create); err != nil {
		if errors.IsTransient(err) {
			return err
		}
		if strings.Contains(err.Error(), "Privileged containers are not allowed") && up.Dev.Docker.Enabled {
			return fmt.Errorf("docker support requires privileged containers. Privileged containers are not allowed in your current cluster")
		}
		if _, ok := err.(errors.UserError); ok {
			return err
		}
		return fmt.Errorf("couldn't activate your development container\n    %s", err.Error())
	}

	if up.isRetry {
		analytics.TrackReconnect(true, up.isSwap)
	}

	up.isRetry = true

	if err := up.forwards(ctx); err != nil {
		if err == errors.ErrSSHConnectError {
			err := up.checkOktetoStartError(ctx, "Failed to connect to your development container")
			if err == errors.ErrLostSyncthing {
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
			return errors.ErrLostSyncthing
		}
		return err
	}

	up.success = true

	go func() {
		output := <-up.cleaned
		log.Debugf("clean command output: %s", output)

		outByCommand := strings.Split(strings.TrimSpace(output), "\n")
		var version, watches, hook string
		if len(outByCommand) >= 3 {
			version, watches, hook = outByCommand[0], outByCommand[1], outByCommand[2]

			if isWatchesConfigurationTooLow(watches) {
				folder := config.GetNamespaceHome(up.Dev.Namespace)
				if utils.GetWarningState(folder, ".remotewatcher") == "" {
					log.Yellow("The value of /proc/sys/fs/inotify/max_user_watches in your cluster nodes is too low.")
					log.Yellow("This can affect file synchronization performance.")
					log.Yellow("Visit https://okteto.com/docs/reference/known-issues/ for more information.")
					if err := utils.SetWarningState(folder, ".remotewatcher", "true"); err != nil {
						log.Infof("failed to set warning remotewatcher state: %s", err.Error())
					}
				}
			}

			if version != model.OktetoBinImageTag {
				log.Yellow("The Okteto CLI version %s uses the init container image %s.", config.VersionString, model.OktetoBinImageTag)
				log.Yellow("Please consider upgrading your init container image %s with the content of %s", up.Dev.InitContainer.Image, model.OktetoBinImageTag)
				log.Infof("Using init image %s instead of default init image (%s)", up.Dev.InitContainer.Image, model.OktetoBinImageTag)
			}

		}
		divertURL := ""
		if up.Dev.Divert != nil {
			username := okteto.GetSanitizedUsername()
			name := diverts.DivertName(username, up.Dev.Divert.Ingress)
			i, err := ingressesv1.Get(ctx, name, up.Dev.Namespace, up.Client)
			if err != nil {
				log.Errorf("error getting diverted ingress %s: %s", name, err.Error())
			} else if len(i.Spec.Rules) > 0 {
				divertURL = i.Spec.Rules[0].Host
			}
		}
		printDisplayContext(up.Dev, divertURL)
		durationActivateUp := time.Since(up.StartTime)
		analytics.TrackDurationActivateUp(durationActivateUp)
		if hook == "yes" {
			log.Information("Running start.sh hook...")
			if err := up.runCommand(ctx, []string{"/var/okteto/cloudbin/start.sh"}); err != nil {
				up.CommandResult <- err
				return
			}
		}
		up.CommandResult <- up.runCommand(ctx, up.Dev.Command.Values)
	}()
	prevError := up.waitUntilExitOrInterrupt()

	up.isRetry = true
	if up.shouldRetry(ctx, prevError) {
		if !up.Dev.PersistentVolumeEnabled() {
			if err := pods.Destroy(ctx, up.Pod.Name, up.Dev.Namespace, up.Client); err != nil {
				return err
			}
		}
		return errors.ErrLostSyncthing
	}

	return prevError
}

func (up *upContext) shouldRetry(ctx context.Context, err error) bool {
	switch err {
	case nil:
		return false
	case errors.ErrLostSyncthing:
		return true
	case errors.ErrCommandFailed:
		return !up.Sy.Ping(ctx, false)
	}

	return false
}

func (up *upContext) devMode(ctx context.Context, k8sObject *model.K8sObject, create bool) error {
	if err := up.createDevContainer(ctx, k8sObject, create); err != nil {
		return err
	}
	return up.waitUntilDevelopmentContainerIsRunning(ctx, k8sObject.ObjectType)
}

func (up *upContext) createDevContainer(ctx context.Context, k8sObject *model.K8sObject, create bool) error {
	spinner := utils.NewSpinner("Activating your development container...")
	spinner.Start()
	up.spinner = spinner
	defer spinner.Stop()

	if err := config.UpdateStateFile(up.Dev, config.Starting); err != nil {
		return err
	}

	if up.Dev.PersistentVolumeEnabled() {
		if err := volumes.CreateForDev(ctx, up.Dev, up.Client); err != nil {
			return err
		}
	}

	resetOnDevContainerStart := up.resetSyncthing || !up.Dev.PersistentVolumeEnabled()
	trList, err := apps.GetTranslations(ctx, up.Dev, k8sObject, resetOnDevContainerStart, up.Client)
	if err != nil {
		return err
	}

	if err := apps.TranslateDevMode(trList, up.Client, up.isOktetoNamespace); err != nil {
		return err
	}

	initSyncErr := <-up.hardTerminate
	if initSyncErr != nil {
		return initSyncErr
	}

	log.Info("create deployment secrets")
	if err := secrets.Create(ctx, up.Dev, up.Client, up.Sy); err != nil {
		return err
	}

	for name := range trList {
		if name == trList[name].K8sObject.Name && create {
			if err := apps.Create(ctx, trList[name].K8sObject, up.Client); err != nil {
				return err
			}
		} else {
			if err := apps.Update(ctx, trList[name].K8sObject, up.Client); err != nil {
				return err
			}
		}
	}

	if create {
		if err := services.CreateDev(ctx, up.Dev, up.Client); err != nil {
			return err
		}
	}

	pod, err := pods.GetDevPodInLoop(ctx, up.Dev, up.Client, create)
	if err != nil {
		return err
	}

	up.Pod = pod

	return nil
}

func (up *upContext) waitUntilDevelopmentContainerIsRunning(ctx context.Context, objectType model.ObjectType) error {
	msg := "Pulling images..."
	if up.Dev.PersistentVolumeEnabled() {
		msg = "Attaching persistent volume..."
		if err := config.UpdateStateFile(up.Dev, config.Attaching); err != nil {
			log.Infof("error updating state: %s", err.Error())
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
			return errors.UserError{E: fmt.Errorf("insufficient resources"),
				Hint: "Increase cluster resources or timeout of resources. More information is available here: https://okteto.com/docs/reference/manifest/#timeout-time-optional"}
		}
		select {
		case event := <-watcherEvents.ResultChan():
			if killing {
				continue
			}
			e, ok := event.Object.(*apiv1.Event)
			if !ok {
				watcherEvents, err = up.Client.CoreV1().Events(up.Dev.Namespace).Watch(ctx, optsWatchEvents)
				if err != nil {
					log.Infof("error watching events: %s", err.Error())
					return err
				}
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
			log.Infof("pod event: %s:%s", e.Reason, e.Message)
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
				log.Success("Persistent volume successfully attached")
				spinner.Update("Pulling images...")
				spinner.Start()
			case "Killing":
				if objectType == model.StatefulsetObjectType {
					killing = true
					continue
				}
				return errors.ErrDevPodDeleted
			case "Started":
				if e.Message == "Started container okteto-init-data" {
					spinner.Update("Initializing persistent volume content...")
				}
			case "Pulling":
				message := getPullingMessage(e.Message, up.Dev.Namespace)
				spinner.Update(fmt.Sprintf("%s...", message))
				if err := config.UpdateStateFile(up.Dev, config.Pulling); err != nil {
					log.Infof("error updating state: %s", err.Error())
				}
			}
		case event := <-watcherPod.ResultChan():
			pod, ok := event.Object.(*apiv1.Pod)
			if !ok {
				watcherPod, err = up.Client.CoreV1().Pods(up.Dev.Namespace).Watch(ctx, optsWatchPod)
				if err != nil {
					log.Infof("error watching pod events: %s", err.Error())
					return err
				}
				continue
			}
			if pod.UID != up.Pod.UID {
				continue
			}

			log.Infof("dev pod %s is now %s", pod.Name, pod.Status.Phase)
			if pod.Status.Phase == apiv1.PodRunning {
				spinner.Stop()
				log.Success("Images successfully pulled")
				return nil
			}
			if pod.DeletionTimestamp != nil {
				return errors.ErrDevPodDeleted
			}
		case <-ctx.Done():
			log.Debug("call to waitUntilDevelopmentContainerIsRunning cancelled")
			return ctx.Err()
		}
	}
}

func getPullingMessage(message, namespace string) string {
	registry, err := okteto.GetRegistry()
	if err != nil {
		return message
	}
	toReplace := fmt.Sprintf("%s/%s", registry, namespace)
	return strings.Replace(message, toReplace, okteto.DevRegistry, 1)
}
