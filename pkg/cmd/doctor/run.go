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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mholt/archives"
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
	"gopkg.in/yaml.v2"
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
func Run(ctx context.Context, dev *model.Dev, devPath string, namespace string, c *kubernetes.Clientset) (string, error) {
	z := archives.Zip{
		SelectiveCompression: true,
		ContinueOnError:      true,
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

	podPath, err := generatePodFile(ctx, dev, namespace, c)
	if err != nil {
		oktetoLog.Infof("failed to get information about the remote dev container: %s", err)
		oktetoLog.Warning(oktetoErrors.ErrNotInDevMode.Error())
	} else {
		defer os.RemoveAll(podPath)
	}

	containerLogsPath, err := generateContainerLogsFolder(ctx, dev, namespace, c)
	if err != nil {
		oktetoLog.Infof("error getting container logs: %s", err)
	} else {
		defer os.RemoveAll(containerLogsPath)
	}

	syncthingLogsPath, err := generateSyncthingLogsFolder(ctx, dev, namespace, c)
	if err != nil {
		oktetoLog.Infof("error getting syncthing logs: %s", err)
	} else {
		defer os.RemoveAll(syncthingLogsPath)
	}

	now := time.Now()
	archiveName := fmt.Sprintf("okteto-doctor-%s.zip", now.Format("20060102150405"))
	files := map[string]string{summaryFilename: "okteto-summary.txt"}
	for _, f := range stignoreFilenames {
		files[f] = filepath.Base(f)
	}

	appLogsPath := filepath.Join(config.GetAppHome(namespace, dev.Name), "okteto.log")
	if filesystem.FileExists(appLogsPath) {
		files[appLogsPath] = "okteto.log"
	}

	if podPath != "" {
		files[podPath] = filepath.Base(podPath)
	}
	if manifestPath != "" {
		files[manifestPath] = filepath.Base(manifestPath)
	}
	if containerLogsPath != "" {
		// Add all log files from the logs directory structure
		err := filepath.Walk(containerLogsPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				// Get the relative path from containerLogsPath
				relPath, err := filepath.Rel(filepath.Dir(containerLogsPath), path)
				if err != nil {
					oktetoLog.Infof("error getting relative path for %s: %s", path, err)
					return nil
				}
				files[path] = relPath
			}
			return nil
		})
		if err != nil {
			oktetoLog.Infof("error walking logs directory: %s", err)
		}
	}

	if syncthingLogsPath != "" {
		// Add all syncthing log files from the syncthing directory structure
		err := filepath.Walk(syncthingLogsPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				// Get the relative path from syncthingLogsPath
				relPath, err := filepath.Rel(filepath.Dir(syncthingLogsPath), path)
				if err != nil {
					oktetoLog.Infof("error getting relative path for %s: %s", path, err)
					return nil
				}
				files[path] = relPath
			}
			return nil
		})
		if err != nil {
			oktetoLog.Infof("error walking syncthing directory: %s", err)
		}
	}

	k8sLogsPath := io.GetK8sLoggerFilePath(config.GetOktetoHome())
	if filesystem.FileExists(k8sLogsPath) {
		files[k8sLogsPath] = "k8s.log"
	}

	archiveFiles, err := archives.FilesFromDisk(ctx, &archives.FromDiskOptions{}, files)
	if err != nil {
		oktetoLog.Infof("error while reading files from disk: %s", err)
		return "", fmt.Errorf("couldn't create archive '%s', please try again: %w", archiveName, err)
	}

	file, err := os.OpenFile(archiveName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("couldn't create archive '%s', please try again: %w", archiveName, err)
	}
	defer file.Close()

	if err := z.Archive(ctx, file, archiveFiles); err != nil {
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
		Image:       "",
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

	for i := range dev.Services {
		dev.Services[i].Environment = nil
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

func generatePodFile(ctx context.Context, dev *model.Dev, namespace string, c kubernetes.Interface) (string, error) {
	oktetoLog.Infof("generating pod file for dev '%s'", dev.Name)

	var devApp apps.App
	if dev.Autocreate {
		devCopy := *dev
		devCopy.Name = model.DevCloneName(dev.Name)
		app, err := apps.Get(ctx, &devCopy, namespace, c)
		if err != nil {
			oktetoLog.Infof("failed to get autocreate app: %s", err)
			return "", err
		}
		devApp = app
	} else {
		app, err := apps.Get(ctx, dev, namespace, c)
		if err != nil {
			oktetoLog.Infof("failed to get app: %s", err)
			return "", err
		}

		if !apps.IsDevModeOn(app) {
			oktetoLog.Infof("app '%s' is not in dev mode", dev.Name)
			return "", oktetoErrors.ErrNotInDevMode
		}

		devApp = app.DevClone()
	}

	if err := devApp.Refresh(ctx, c); err != nil {
		oktetoLog.Infof("failed to refresh app: %s", err)
		return "", err
	}

	pod, err := devApp.GetRunningPod(ctx, c)
	if err != nil {
		oktetoLog.Infof("failed to get running pod: %s", err)
		return "", err
	}

	if pod == nil {
		oktetoLog.Infof("no running pod found for dev '%s'", dev.Name)
		return "", oktetoErrors.ErrNotFound
	}

	oktetoLog.Infof("found running pod '%s'", pod.Name)

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

func generateContainerLogsFolder(ctx context.Context, dev *model.Dev, namespace string, c kubernetes.Interface) (string, error) {
	oktetoLog.Infof("generating container logs for dev '%s'", dev.Name)

	var devApp apps.App
	if dev.Autocreate {
		devCopy := *dev
		devCopy.Name = model.DevCloneName(dev.Name)
		app, err := apps.Get(ctx, &devCopy, namespace, c)
		if err != nil {
			oktetoLog.Infof("failed to get autocreate app: %s", err)
			return "", err
		}
		devApp = app
	} else {
		app, err := apps.Get(ctx, dev, namespace, c)
		if err != nil {
			oktetoLog.Infof("failed to get app: %s", err)
			return "", err
		}

		if !apps.IsDevModeOn(app) {
			oktetoLog.Infof("app '%s' is not in dev mode", dev.Name)
			return "", oktetoErrors.ErrNotInDevMode
		}

		// Get the dev clone
		devApp = app.DevClone()
	}

	// Refresh to get latest state
	if err := devApp.Refresh(ctx, c); err != nil {
		oktetoLog.Infof("failed to refresh app: %s", err)
		return "", err
	}

	// Get the running pod
	pod, err := devApp.GetRunningPod(ctx, c)
	if err != nil {
		oktetoLog.Infof("failed to get running pod: %s", err)
		return "", err
	}

	oktetoLog.Infof("retrieving logs from pod '%s'", pod.Name)

	// Create temp directory for logs
	tempdir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", fmt.Errorf("error creating temp dir: %w", err)
	}
	logsDir := filepath.Join(tempdir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return "", fmt.Errorf("error creating logs dir: %w", err)
	}

	// Create subdirectories for init containers and main containers
	initContainersDir := filepath.Join(logsDir, "initContainers")
	containersDir := filepath.Join(logsDir, "containers")
	if err := os.MkdirAll(initContainersDir, 0755); err != nil {
		return "", fmt.Errorf("error creating initContainers dir: %w", err)
	}
	if err := os.MkdirAll(containersDir, 0755); err != nil {
		return "", fmt.Errorf("error creating containers dir: %w", err)
	}

	// Get logs from all init containers
	for _, container := range pod.Spec.InitContainers {
		containerName := container.Name
		oktetoLog.Infof("retrieving logs from init container '%s'", containerName)

		containerLogs, err := pods.ContainerLogs(ctx, containerName, pod.Name, namespace, false, c)
		if err != nil {
			oktetoLog.Infof("error getting logs for init container '%s': %s", containerName, err)
			continue
		}

		logFilePath := filepath.Join(initContainersDir, fmt.Sprintf("%s.log", containerName))
		if err := os.WriteFile(logFilePath, []byte(containerLogs), 0600); err != nil {
			oktetoLog.Infof("error writing logs for init container '%s': %s", containerName, err)
		}
	}

	// Get logs from all main containers
	for _, container := range pod.Spec.Containers {
		containerName := container.Name
		oktetoLog.Infof("retrieving logs from container '%s'", containerName)

		containerLogs, err := pods.ContainerLogs(ctx, containerName, pod.Name, namespace, false, c)
		if err != nil {
			oktetoLog.Infof("error getting logs for container '%s': %s", containerName, err)
			continue
		}

		logFilePath := filepath.Join(containersDir, fmt.Sprintf("%s.log", containerName))
		if err := os.WriteFile(logFilePath, []byte(containerLogs), 0600); err != nil {
			oktetoLog.Infof("error writing logs for container '%s': %s", containerName, err)
		}
	}

	return logsDir, nil
}

func generateSyncthingLogsFolder(ctx context.Context, dev *model.Dev, namespace string, c kubernetes.Interface) (string, error) {
	oktetoLog.Infof("generating syncthing logs for dev '%s'", dev.Name)

	// Create temp directory for syncthing logs
	tempdir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", fmt.Errorf("error creating temp dir: %w", err)
	}
	syncthingDir := filepath.Join(tempdir, "syncthing")
	if err := os.MkdirAll(syncthingDir, 0755); err != nil {
		return "", fmt.Errorf("error creating syncthing dir: %w", err)
	}

	// Create subdirectories for local and remote
	localDir := filepath.Join(syncthingDir, "local")
	remoteDir := filepath.Join(syncthingDir, "remote")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return "", fmt.Errorf("error creating local dir: %w", err)
	}
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		return "", fmt.Errorf("error creating remote dir: %w", err)
	}

	// Copy local syncthing logs
	localSyncthingLogPath := syncthing.GetLogFile(namespace, dev.Name)
	if filesystem.FileExists(localSyncthingLogPath) {
		oktetoLog.Infof("copying local syncthing logs")
		localLogs, err := os.ReadFile(localSyncthingLogPath)
		if err != nil {
			oktetoLog.Infof("error reading local syncthing logs: %s", err)
		} else {
			destPath := filepath.Join(localDir, "syncthing.log")
			if err := os.WriteFile(destPath, localLogs, 0600); err != nil {
				oktetoLog.Infof("error writing local syncthing logs: %s", err)
			}
		}
	}

	// Get remote syncthing logs
	var devApp apps.App
	if dev.Autocreate {
		devCopy := *dev
		devCopy.Name = model.DevCloneName(dev.Name)
		app, err := apps.Get(ctx, &devCopy, namespace, c)
		if err != nil {
			oktetoLog.Infof("failed to get autocreate app: %s", err)
			return syncthingDir, nil
		}
		devApp = app
	} else {
		app, err := apps.Get(ctx, dev, namespace, c)
		if err != nil {
			oktetoLog.Infof("failed to get app: %s", err)
			return syncthingDir, nil
		}

		if !apps.IsDevModeOn(app) {
			oktetoLog.Infof("app '%s' is not in dev mode", dev.Name)
			return syncthingDir, nil
		}

		devApp = app.DevClone()
	}

	// Refresh to get latest state
	if err := devApp.Refresh(ctx, c); err != nil {
		oktetoLog.Infof("failed to refresh app: %s", err)
		return syncthingDir, nil
	}

	// Get the running pod
	pod, err := devApp.GetRunningPod(ctx, c)
	if err != nil {
		oktetoLog.Infof("failed to get running pod: %s", err)
		return syncthingDir, nil
	}

	oktetoLog.Infof("retrieving remote syncthing logs from pod '%s'", pod.Name)

	// Try to get logs from syncthing container (usually the dev container)
	remoteLogs, err := pods.ContainerLogs(ctx, dev.Container, pod.Name, namespace, false, c)
	if err != nil {
		oktetoLog.Infof("error getting remote syncthing logs: %s", err)
	} else {
		destPath := filepath.Join(remoteDir, "syncthing.log")
		if err := os.WriteFile(destPath, []byte(remoteLogs), 0600); err != nil {
			oktetoLog.Infof("error writing remote syncthing logs: %s", err)
		}
	}

	return syncthingDir, nil
}
