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
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"k8s.io/client-go/kubernetes"
)

//Run runs the "okteto status" sequence
func Run(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset) error {
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
		return err
	}
	defer os.Remove(summaryFilename)

	remoteLogsPath, err := generateRemoteSyncthingLogsFile(ctx, dev, c)
	if err != nil {
		log.Yellow("Failed to query remote syncthing logs: %s", err)
	}
	if remoteLogsPath != "" {
		defer os.RemoveAll(remoteLogsPath)
	}

	now := time.Now()
	archiveName := fmt.Sprintf("okteto-doctor-%s.zip", now.Format("20060102150405"))
	files := []string{summaryFilename, filepath.Join(config.GetHome(), "okteto.log"), config.GetSyncthingLogFile(dev.Namespace, dev.Name)}
	if remoteLogsPath != "" {
		files = append(files, remoteLogsPath)
	}
	if err := z.Archive(files, archiveName); err != nil {
		log.Infof("error while archiving: %s", err)
		return fmt.Errorf("couldn't create archive, please try again")
	}

	log.Information("Your doctor file is available at %s", archiveName)
	return nil
}

func generateSummaryFile() (string, error) {
	tempdir, _ := ioutil.TempDir("", "")
	summaryPath := path.Join(tempdir, "okteto-summary.txt")
	fileSummary, err := os.OpenFile(summaryPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(fileSummary, "version=%s\n", config.VersionString)
	fmt.Fprintf(fileSummary, "os=%s\n", runtime.GOOS)
	if err := fileSummary.Sync(); err != nil {
		return "", err
	}
	fileSummary.Close()
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
	fmt.Fprint(fileRemoteLog, remoteLogs)
	if err := fileRemoteLog.Sync(); err != nil {
		return "", err
	}
	fileRemoteLog.Close()
	return remoteLogsPath, nil
}

func getDevPodLogs(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset) (string, error) {
	p, err := pods.GetDevPod(ctx, dev, c, false)
	if err != nil {
		return "", err
	}
	if len(dev.Container) == 0 {
		dev.Container = p.Spec.Containers[0].Name
	}
	return pods.ContainerLogs(dev.Container, p, dev.Namespace, c)
}
