// Copyright 2020 The Okteto Authors
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
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mholt/archiver"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"k8s.io/client-go/kubernetes"
)

//Run runs the "okteto status" sequence
func Run(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset) (string, error) {
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

	remoteLogsPath, err := generateRemoteSyncthingLogsFile(ctx, dev, c)
	if err != nil {
		log.Debugf("error getting remote syncthing logs: %s", err)
		log.Yellow(errors.ErrNotInDevMode.Error())
	}
	defer os.RemoveAll(remoteLogsPath)

	now := time.Now()
	archiveName := fmt.Sprintf("okteto-doctor-%s.zip", now.Format("20060102150405"))
	files := []string{summaryFilename}
	if model.FileExists(filepath.Join(config.GetOktetoHome(), "okteto.log")) {
		files = append(files, filepath.Join(config.GetOktetoHome(), "okteto.log"))
	}
	if model.FileExists(config.GetSyncthingLogFile(dev.Namespace, dev.Name)) {
		files = append(files, config.GetSyncthingLogFile(dev.Namespace, dev.Name))
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
	summaryPath := path.Join(tempdir, "okteto-summary.txt")
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

func generateRemoteSyncthingLogsFile(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset) (string, error) {
	remoteLogs, err := getDevPodLogs(ctx, dev, c)
	if err != nil {
		return "", err
	}

	tempdir, _ := ioutil.TempDir("", "")
	remoteLogsPath := path.Join(tempdir, "remote-syncthing.log")
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

func getDevPodLogs(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset) (string, error) {
	p, err := pods.GetDevPod(ctx, dev, c, false)
	if err != nil {
		return "", err
	}
	if p == nil {
		return "", errors.ErrNotFound
	}
	if len(dev.Container) == 0 {
		dev.Container = p.Spec.Containers[0].Name
	}
	return pods.ContainerLogs(dev.Container, p, dev.Namespace, c)
}
