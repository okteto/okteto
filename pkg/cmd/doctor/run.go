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

package doctor

import (
	"compress/flate"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mholt/archiver/v3"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/pods"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/okteto/okteto/pkg/syncthing"
	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// PodInfo info collected for pods
type PodInfo struct {
	CPU        string               `yaml:"cpu,omitempty"`
	Memory     string               `yaml:"memory,omitempty"`
	Conditions []apiv1.PodCondition `yaml:"conditions,omitempty"`
}

// Run runs the "okteto status" sequence
func Run(ctx context.Context, dev *model.Dev, devPath string, c *kubernetes.Clientset) (string, error) {
	z := archiver.Zip{
		CompressionLevel:       flate.DefaultCompression,
		MkdirAll:               true,
		SelectiveCompression:   true,
		ContinueOnError:        true,
		OverwriteExisting:      true,
		ImplicitTopLevelFolder: true,
	}

	summaryFilename, err := generateSummaryFile()
	if err != nil {
		return "", err
	}
	defer os.Remove(summaryFilename)

	stignoreFilenames := generateStignoreFiles(dev)

	manifestPath, err := generateManifestFile(devPath)
	if err != nil {
		oktetoLog.Infof("failed to get information for okteto manifest: %s", err)
	}
	defer os.RemoveAll(manifestPath)

	podPath, err := generatePodFile(ctx, dev, c)
	if err != nil {
		oktetoLog.Infof("failed to get information about the remote dev container: %s", err)
		oktetoLog.Warning(oktetoErrors.ErrNotInDevMode.Error())
	} else {
		defer os.RemoveAll(podPath)
	}

	remoteLogsPath, err := generateRemoteSyncthingLogsFile(ctx, dev, c)
	if err != nil {
		oktetoLog.Infof("error getting remote syncthing logs: %s", err)
	} else {
		defer os.RemoveAll(remoteLogsPath)
	}

	now := time.Now()
	archiveName := fmt.Sprintf("okteto-doctor-%s.zip", now.Format("20060102150405"))
	files := []string{summaryFilename}
	files = append(files, stignoreFilenames...)

	appLogsPath := filepath.Join(config.GetAppHome(dev.Namespace, dev.Name), "okteto.log")
	if filesystem.FileExists(appLogsPath) {
		files = append(files, appLogsPath)
	}

	if filesystem.FileExists(syncthing.GetLogFile(dev.Namespace, dev.Name)) {
		files = append(files, syncthing.GetLogFile(dev.Namespace, dev.Name))
	}
	if podPath != "" {
		files = append(files, podPath)
	}
	if manifestPath != "" {
		files = append(files, manifestPath)
	}
	if remoteLogsPath != "" {
		files = append(files, remoteLogsPath)
	}

	k8sLogsPath := io.GetK8sLoggerFilePath(config.GetOktetoHome())
	if filesystem.FileExists(k8sLogsPath) {
		files = append(files, k8sLogsPath)
	}

	if err := z.Archive(files, archiveName); err != nil {
		oktetoLog.Infof("error while archiving: %s", err)
		return "", fmt.Errorf("couldn't create archive '%s', please try again: %w", archiveName, err)
	}

	return archiveName, nil
}

func generateSummaryFile() (string, error) {
	tempdir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", fmt.Errorf("error creating temp dir: %w", err)
	}
	summaryPath := filepath.Join(tempdir, "okteto-summary.txt")
	fileSummary, err := os.OpenFile(summaryPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := fileSummary.Close(); err != nil {
			oktetoLog.Debugf("Error closing file %s: %s", summaryPath, err)
		}
	}()
	fmt.Fprintf(fileSummary, "version=%s\nos=%s\narch=%s\n", config.VersionString, runtime.GOOS, runtime.GOARCH)
	if err := fileSummary.Sync(); err != nil {
		return "", err
	}
	return summaryPath, nil
}

func generateStignoreFiles(dev *model.Dev) []string {
	result := []string{}
	for i := range dev.Sync.Folders {
		absBasename, err := filepath.Abs(dev.Sync.Folders[i].LocalPath)
		if err != nil {
			oktetoLog.Infof("error getting absolute path of localPath %s: %s", dev.Sync.Folders[i].LocalPath, err.Error())
			continue
		}
		stignoreFile := filepath.Join(absBasename, ".stignore")
		if _, err := os.Stat(stignoreFile); !os.IsNotExist(err) {
			result = append(result, stignoreFile)
		}
	}
	return result
}

func generateManifestFile(devPath string) (string, error) {
	tempdir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}
	manifestFilename := filepath.Join(tempdir, "okteto.yml")
	manifestFile, err := os.OpenFile(manifestFilename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := manifestFile.Close(); err != nil {
			oktetoLog.Debugf("Error closing file %s: %s", manifestFilename, err)
		}
	}()

	b, err := os.ReadFile(devPath)
	if err != nil {
		return "", err
	}

	dev := &model.Dev{
		Image:       &build.Info{},
		Push:        &build.Info{},
		Environment: make([]env.Var, 0),
		Secrets:     make([]model.Secret, 0),
		Forward:     make([]forward.Forward, 0),
		Volumes:     make([]model.Volume, 0),
		Sync: model.Sync{
			Folders: make([]model.SyncFolder, 0),
		},
		Services: make([]*model.Dev, 0),
	}

	if err := yaml.Unmarshal(b, dev); err != nil {
		return "", err
	}

	dev.Environment = nil

	if dev.Image != nil {
		dev.Image.Args = nil
	}

	if dev.Push != nil {
		dev.Push.Args = nil
	}

	for i := range dev.Services {
		dev.Services[i].Environment = nil

		if dev.Services[i].Image != nil {
			dev.Services[i].Image.Args = nil
		}

		if dev.Services[i].Push != nil {
			dev.Services[i].Push.Args = nil
		}
	}

	marshalled, err := yaml.Marshal(dev)
	if err != nil {
		return "", err
	}
	fmt.Fprint(manifestFile, string(marshalled))
	if err := manifestFile.Sync(); err != nil {
		return "", err
	}
	return manifestFilename, nil
}

func generatePodFile(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset) (string, error) {
	app, err := apps.Get(ctx, dev, dev.Namespace, c)
	if err != nil {
		return "", err
	}
	devApp := app.DevClone()
	if err := devApp.Refresh(ctx, c); err != nil {
		return "", err
	}
	pod, err := devApp.GetRunningPod(ctx, c)
	if err != nil {
		return "", err
	}

	if pod == nil {
		return "", oktetoErrors.ErrNotFound
	}

	tempdir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}
	podFilename := filepath.Join(tempdir, "pod.yaml")
	podFile, err := os.OpenFile(podFilename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := podFile.Close(); err != nil {
			oktetoLog.Debugf("Error closing file %s: %s", podFilename, err)
		}
	}()

	devContainer := apps.GetDevContainer(&pod.Spec, dev.Container)
	cpu := "unlimited"
	memory := "unlimited"
	limits := devContainer.Resources.Limits
	if limits != nil {
		if v, ok := limits[apiv1.ResourceCPU]; ok {
			cpu = v.String()
		}
		if v, ok := limits[apiv1.ResourceMemory]; ok {
			memory = v.String()
		}
	}
	podInfo := PodInfo{
		CPU:        cpu,
		Memory:     memory,
		Conditions: pod.Status.Conditions,
	}
	marshalled, err := yaml.Marshal(podInfo)
	if err != nil {
		return "", err
	}
	fmt.Fprint(podFile, string(marshalled))
	if err := podFile.Sync(); err != nil {
		return "", err
	}
	return podFilename, nil
}

func generateRemoteSyncthingLogsFile(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset) (string, error) {
	app, err := apps.Get(ctx, dev, dev.Namespace, c)
	if err != nil {
		return "", err
	}
	pod, err := app.GetRunningPod(ctx, c)
	if err != nil {
		return "", err
	}

	remoteLogs, err := pods.ContainerLogs(ctx, dev.Container, pod.Name, dev.Namespace, false, c)
	if err != nil {
		return "", err
	}

	tempdir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", fmt.Errorf("error creating temp dir: %w", err)
	}
	remoteLogsPath := filepath.Join(tempdir, "remote-syncthing.log")
	fileRemoteLog, err := os.OpenFile(remoteLogsPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := fileRemoteLog.Close(); err != nil {
			oktetoLog.Debugf("Error closing file %s: %s", remoteLogsPath, err)
		}
	}()

	fmt.Fprint(fileRemoteLog, remoteLogs)
	if err := fileRemoteLog.Sync(); err != nil {
		return "", err
	}
	return remoteLogsPath, nil
}
