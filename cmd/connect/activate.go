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
	"bufio"
	"context"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/kubernetes"
)

// activateLoop runs activate() in a retry loop, sending any terminal error to c.Exit.
func (c *connectContext) activateLoop() {
	defer func() {
		if err := config.DeleteStateFile(c.Dev.Name, c.Namespace); err != nil {
			oktetoLog.Infof("failed to delete state file: %s", err)
		}
	}()

	t := time.NewTicker(1 * time.Second)
	defer t.Stop()

	for {
		if c.isRetry {
			oktetoLog.Infof("waiting for shutdown sequence to finish")
			<-c.ShutdownCompleted
			oktetoLog.Yellow("Connection lost to your development container, reconnecting...")
		}

		err := c.activate()
		if err != nil {
			oktetoLog.Infof("activate failed: %s", err)

			if oktetoErrors.IsTransient(err) {
				<-t.C
				continue
			}

			if err == oktetoErrors.ErrLostSyncthing {
				continue
			}

			c.Exit <- err
			return
		}

		c.Exit <- nil
		return
	}
}

func (c *connectContext) activate() error {
	if err := config.UpdateStateFile(c.Dev.Name, c.Namespace, config.Activating); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.Cancel = cancel
	c.ShutdownCompleted = make(chan bool, 1)
	c.Sy = nil
	defer func() {
		c.shutdown()
	}()

	c.Disconnect = make(chan error, 1)
	c.hardTerminate = make(chan error, 1)

	k8sClient, _, err := c.K8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}

	app, err := getApp(ctx, c.Dev, c.Namespace, k8sClient)
	create := false
	if err != nil {
		return err
	}

	if c.isRetry && !apps.IsDevModeOn(app) {
		oktetoLog.Information("Development container has been deactivated")
		return nil
	}

	if err := app.RestoreOriginal(); err != nil {
		return err
	}

	if err := apps.ValidateMountPaths(app.PodSpec(), c.Dev); err != nil {
		return err
	}

	go func() {
		if err := c.initializeSyncthing(); err != nil {
			oktetoLog.Infof("could not initialize syncthing: %s", err)
		}
	}()

	if err := c.setDevContainer(app); err != nil {
		return err
	}

	if err := c.devMode(ctx, app, create); err != nil {
		if _, ok := err.(oktetoErrors.UserError); ok {
			return err
		}
		return fmt.Errorf("couldn't activate your development container\n    %w", err)
	}

	c.isRetry = true

	if err := c.sync(ctx); err != nil {
		if c.shouldRetry(err) {
			return oktetoErrors.ErrLostSyncthing
		}
		return err
	}

	c.success = true

	oktetoLog.Println(fmt.Sprintf("    %s   %s", oktetoLog.BlueString("Context:"), okteto.RemoveSchema(okteto.GetContext().Name)))
	oktetoLog.Println(fmt.Sprintf("    %s %s", oktetoLog.BlueString("Namespace:"), c.Namespace))
	oktetoLog.Println(fmt.Sprintf("    %s      %s", oktetoLog.BlueString("Name:"), c.Dev.Name))
	oktetoLog.Println(fmt.Sprintf("    %s    %s", oktetoLog.BlueString("Workdir:"), c.Dev.Sync.Folders[0].RemotePath))
	oktetoLog.Success("Files synchronized")
	oktetoLog.Information("The dev container is running. Press Ctrl+C to stop syncing (the container stays running).")

	prevErr := c.waitUntilExitOrInterrupt(ctx)
	if c.shouldRetry(prevErr) {
		return oktetoErrors.ErrLostSyncthing
	}
	return prevErr
}

func (c *connectContext) shouldRetry(err error) bool {
	switch err {
	case nil:
		return false
	case oktetoErrors.ErrLostSyncthing:
		return true
	}
	return false
}

func (c *connectContext) waitUntilExitOrInterrupt(ctx context.Context) error {
	for {
		select {
		case err := <-c.Disconnect:
			if err == oktetoErrors.ErrInsufficientSpace {
				return oktetoErrors.UserError{
					E:    err,
					Hint: "The synchronization service is running out of space.",
				}
			}
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *connectContext) devMode(ctx context.Context, app apps.App, create bool) error {
	return c.createDevContainer(ctx, app, create)
}

func (c *connectContext) createDevContainer(ctx context.Context, app apps.App, create bool) error {
	oktetoLog.Spinner("Activating your development container...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	if err := config.UpdateStateFile(c.Dev.Name, c.Namespace, config.Starting); err != nil {
		return err
	}

	k8sClient, _, err := c.K8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}

	trMap, err := apps.GetTranslations(ctx, c.Namespace, c.Manifest.Name, c.Dev, app, false, k8sClient)
	if err != nil {
		return err
	}
	c.Translations = trMap

	if err := apps.TranslateDevMode(trMap); err != nil {
		return err
	}

	// Inject Claude Code init container into the dev pod spec.
	for _, tr := range trMap {
		if tr.MainDev == tr.Dev {
			apps.TranslateClaudeCodeInitContainer(tr.DevApp.PodSpec())
			apps.MountClaudeCodeBin(&tr.DevApp.PodSpec().Containers[0])
		}
	}

	initSyncErr := <-c.hardTerminate
	if initSyncErr != nil {
		return initSyncErr
	}

	oktetoLog.Info("create deployment secrets")
	if err := secrets.Create(ctx, c.Dev, c.Namespace, k8sClient, c.Sy); err != nil {
		return err
	}

	dd := newDevDeployer(trMap, k8sClient)
	if err := dd.deployMainDev(ctx); err != nil {
		return err
	}
	if err := dd.deployDevServices(ctx); err != nil {
		return err
	}

	_ = create // no service creation needed for manifest-free connect

	pod, err := apps.GetRunningPodInLoop(ctx, c.Dev, dd.mainTranslation.DevApp, k8sClient)
	if err != nil {
		return err
	}
	c.Pod = pod
	return nil
}

func (c *connectContext) setDevContainer(app apps.App) error {
	devContainer := apps.GetDevContainer(app.PodSpec(), c.Dev.Container)
	if devContainer == nil {
		return fmt.Errorf("container '%s' does not exist in deployment '%s'", c.Dev.Container, c.Dev.Name)
	}
	c.Dev.Container = devContainer.Name
	if c.Dev.Image == "" {
		c.Dev.Image = devContainer.Image
	}
	return nil
}

// devDeployer mirrors cmd/up.devDeployer but lives in this package to avoid cross-package coupling.
type devDeployer struct {
	translations    map[string]*apps.Translation
	k8sClient       kubernetes.Interface
	mainTranslation *apps.Translation
	devTranslations []*apps.Translation
}

func newDevDeployer(translations map[string]*apps.Translation, k8sClient kubernetes.Interface) *devDeployer {
	var mainTranslation *apps.Translation
	devTranslations := make([]*apps.Translation, 0)

	for _, tr := range translations {
		if tr.MainDev == tr.Dev {
			mainTranslation = tr
		} else {
			devTranslations = append(devTranslations, tr)
		}
	}

	return &devDeployer{
		translations:    translations,
		k8sClient:       k8sClient,
		mainTranslation: mainTranslation,
		devTranslations: devTranslations,
	}
}

func (dd *devDeployer) deployMainDev(ctx context.Context) error {
	return dd.deploy(ctx, dd.mainTranslation)
}

func (dd *devDeployer) deployDevServices(ctx context.Context) error {
	for _, tr := range dd.devTranslations {
		if err := dd.deploy(ctx, tr); err != nil {
			return err
		}
	}
	return nil
}

func (dd *devDeployer) deploy(ctx context.Context, tr *apps.Translation) error {
	delete(tr.DevApp.ObjectMeta().Annotations, model.DeploymentRevisionAnnotation)
	if err := tr.DevApp.Deploy(ctx, dd.k8sClient); err != nil {
		return err
	}
	return tr.App.Deploy(ctx, dd.k8sClient)
}

// getApp looks up the app by name.
func getApp(ctx context.Context, dev *model.Dev, namespace string, k8sClient kubernetes.Interface) (apps.App, error) {
	app, err := apps.Get(ctx, dev, namespace, k8sClient)
	if err != nil {
		if !oktetoErrors.IsNotFound(err) {
			return nil, err
		}
		return nil, oktetoErrors.UserError{
			E:    fmt.Errorf("application '%s' not found in namespace '%s'", dev.Name, namespace),
			Hint: "Verify that your application is running and your okteto context is pointing to the right namespace",
		}
	}
	return app, nil
}

// inferDevFromDeployment fetches the named Deployment or StatefulSet, extracts the working
// directory from its first container, and builds a minimal *model.Manifest suitable for the
// activation pipeline.
func inferDevFromDeployment(ctx context.Context, name, namespace string, opts *Options, k8sClient kubernetes.Interface) (*model.Manifest, error) {
	var (
		workdir string
		image   string
	)

	// Try Deployment first, then StatefulSet.
	d, err := deployments.Get(ctx, name, namespace, k8sClient)
	if err == nil {
		appWrapper := apps.NewDeploymentApp(d)
		workdir, image, err = getContainerInfo(appWrapper)
		if err != nil {
			return nil, err
		}
	} else {
		sfs, sfsErr := statefulsets.Get(ctx, name, namespace, k8sClient)
		if sfsErr != nil {
			return nil, oktetoErrors.UserError{
				E:    fmt.Errorf("application '%s' not found in namespace '%s'", name, namespace),
				Hint: "Verify that your application is running and your okteto context is pointing to the right namespace",
			}
		}
		appWrapper := apps.NewStatefulSetApp(sfs)
		workdir, image, err = getContainerInfo(appWrapper)
		if err != nil {
			return nil, err
		}
	}

	// Override image from flag if provided.
	if opts.Image != "" {
		image = opts.Image
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	dev := model.NewDev()
	dev.Name = name
	dev.Image = image
	dev.Sync.Folders = []model.SyncFolder{
		{LocalPath: cwd, RemotePath: workdir},
	}

	if err := applyEnvOverrides(&dev.Environment, opts.Envs); err != nil {
		return nil, err
	}

	if err := dev.SetDefaults(); err != nil {
		return nil, fmt.Errorf("failed to set dev defaults: %w", err)
	}

	manifest := &model.Manifest{
		Name: name,
		Dev:  model.ManifestDevs{name: dev},
	}
	return manifest, nil
}

// getContainerInfo returns workingDir and image from the first container of a K8s App.
func getContainerInfo(app apps.App) (workdir, image string, err error) {
	podSpec := app.PodSpec()
	if podSpec == nil || len(podSpec.Containers) == 0 {
		return "", "", fmt.Errorf("deployment has no containers")
	}
	c := podSpec.Containers[0]
	if c.WorkingDir == "" {
		return "", "", oktetoErrors.UserError{
			E:    fmt.Errorf("could not infer working directory for deployment %q", app.ObjectMeta().Name),
			Hint: "Set workingDir in your container spec. Support for a --remote-path flag is planned.",
		}
	}
	return c.WorkingDir, c.Image, nil
}

// addStignoreSecrets mirrors cmd/up/stignore.go:addStignoreSecrets.
func addStignoreSecrets(dev *model.Dev, namespace string) error {
	output := ""
	for i, folder := range dev.Sync.Folders {
		stignorePath := filepath.Join(folder.LocalPath, ".stignore")
		infile, err := os.Open(stignorePath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return oktetoErrors.UserError{
				E:    err,
				Hint: "Update the 'sync' field of your okteto manifest to point to a valid directory path",
			}
		}
		defer func() {
			if err := infile.Close(); err != nil {
				oktetoLog.Debugf("Error closing file %s: %s", stignorePath, err)
			}
		}()

		stignoreName := fmt.Sprintf(".stignore-%d", i+1)
		transformedPath := filepath.Join(config.GetAppHome(namespace, dev.Name), stignoreName)
		outfile, err := os.OpenFile(transformedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		defer func() {
			if err := outfile.Close(); err != nil {
				oktetoLog.Infof("Error closing file %s: %s", transformedPath, err)
			}
		}()

		reader := bufio.NewReader(infile)
		writer := bufio.NewWriter(outfile)
		defer writer.Flush()

		for {
			bytes, _, err := reader.ReadLine()
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			line := strings.TrimSpace(string(bytes))
			if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
				continue
			}
			if !strings.HasPrefix(line, "!") && !strings.Contains(line, "(?d)") {
				line = fmt.Sprintf("(?d)%s", line)
			}
			if _, err := writer.WriteString(fmt.Sprintf("%s\n", line)); err != nil {
				return err
			}
			output = fmt.Sprintf("%s\n%s", output, line)
		}

		dev.Secrets = append(dev.Secrets, model.Secret{
			LocalPath:  transformedPath,
			RemotePath: path.Join(folder.RemotePath, ".stignore"),
			Mode:       0644,
		})
	}
	dev.Metadata.Annotations[model.OktetoStignoreAnnotation] = fmt.Sprintf("%x", sha512.Sum512([]byte(output)))
	return nil
}

// addSyncFieldHash mirrors cmd/up/stignore.go:addSyncFieldHash.
func addSyncFieldHash(dev *model.Dev) error {
	output, err := json.Marshal(dev.Sync)
	if err != nil {
		return err
	}
	dev.Metadata.Annotations[model.OktetoSyncAnnotation] = fmt.Sprintf("%x", sha512.Sum512(output))
	return nil
}
