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

package doctor

import (
	"compress/flate"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mholt/archiver"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/syncthing"
	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

//PodInfo info collected for pods
type PodInfo struct {
	CPU        string               `yaml:"cpu,omitempty"`
	Memory     string               `yaml:"memory,omitempty"`
	Conditions []apiv1.PodCondition `yaml:"conditions,omitempty"`
}

//Run runs the "okteto status" sequence
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

	manifestPath, err := generateManifestFile(ctx, devPath)
	if err != nil {
		log.Infof("failed to get information for okteto manifest: %s", err)
	}
	defer os.RemoveAll(manifestPath)

	podPath, err := generatePodFile(ctx, dev, c)
	if err != nil {
		log.Infof("failed to get information about the remote dev container: %s", err)
		log.Yellow(errors.ErrNotInDevMode.Error())
	} else {
		defer os.RemoveAll(podPath)
	}

	remoteLogsPath, err := generateRemoteSyncthingLogsFile(ctx, dev, c)
	if err != nil {
		log.Infof("error getting remote syncthing logs: %s", err)
	} else {
		defer os.RemoveAll(remoteLogsPath)
	}

	now := time.Now()
	archiveName := fmt.Sprintf("okteto-doctor-%s.zip", now.Format("20060102150405"))
	files := []string{summaryFilename}
	files = append(files, stignoreFilenames...)

	deploymentLogsPath := filepath.Join(config.GetDeploymentHome(dev.Namespace, dev.Name), "okteto.log")
	if model.FileExists(deploymentLogsPath) {
		files = append(files, deploymentLogsPath)
	}

	if model.FileExists(syncthing.GetLogFile(dev.Namespace, dev.Name)) {
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
	if err := z.Archive(files, archiveName); err != nil {
		log.Infof("error while archiving: %s", err)
		return "", fmt.Errorf("couldn't create archive '%s', please try again: %s", archiveName, err)
	}

	return archiveName, nil
}

func generateSummaryFile() (string, error) {
	tempdir, _ := ioutil.TempDir("", "")
	summaryPath := filepath.Join(tempdir, "okteto-summary.txt")
	fileSummary, err := os.OpenFile(summaryPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	defer fileSummary.Close()
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
			log.Infof("error getting absolute path of localPath %s: %s", dev.Sync.Folders[i].LocalPath, err.Error())
			continue
		}
		stignoreFile := filepath.Join(absBasename, ".stignore")
		if _, err := os.Stat(stignoreFile); !os.IsNotExist(err) {
			result = append(result, stignoreFile)
		}
	}
	return result
}

func generateManifestFile(ctx context.Context, devPath string) (string, error) {
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}
	manifestFilename := filepath.Join(tempdir, "okteto.yml")
	manifestFile, err := os.OpenFile(manifestFilename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	defer manifestFile.Close()

	b, err := ioutil.ReadFile(devPath)
	if err != nil {
		return "", err
	}

	dev := &model.Dev{
		Image:       &model.BuildInfo{},
		Push:        &model.BuildInfo{},
		Environment: make([]model.EnvVar, 0),
		Secrets:     make([]model.Secret, 0),
		Forward:     make([]model.Forward, 0),
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
	pod, err := pods.GetDevPod(ctx, dev, c, false)
	if err != nil {
		return "", err
	}

	if pod == nil {
		return "", errors.ErrNotFound
	}

	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}
	podFilename := filepath.Join(tempdir, "pod.yaml")
	podFile, err := os.OpenFile(podFilename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	defer podFile.Close()

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
	remoteLogs, err := pods.GetDevPodLogs(ctx, dev, true, c)
	if err != nil {
		return "", err
	}

	tempdir, _ := ioutil.TempDir("", "")
	remoteLogsPath := filepath.Join(tempdir, "remote-syncthing.log")
	fileRemoteLog, err := os.OpenFile(remoteLogsPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	defer fileRemoteLog.Close()
	fmt.Fprint(fileRemoteLog, remoteLogs)
	if err := fileRemoteLog.Sync(); err != nil {
		return "", err
	}
	return remoteLogsPath, nil
}
