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
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/spf13/afero"
)

const (
	defaultProgressBarWidth = 40
	totalProgressValue      = 100
)

func (up *upContext) initializeSyncthing() error {
	sy, err := syncthing.New(up.Dev)
	if err != nil {
		return err
	}
	sy.ResetDatabase = up.resetSyncthing
	up.Sy = sy

	oktetoLog.Infof("local syncthing initialized: gui -> %d, sync -> %d", up.Sy.LocalGUIPort, up.Sy.LocalPort)
	oktetoLog.Infof("remote syncthing initialized: gui -> %d, sync -> %d", up.Sy.RemoteGUIPort, up.Sy.RemotePort)

	if err := up.Sy.SaveConfig(up.Dev); err != nil {
		oktetoLog.Infof("error saving syncthing object: %s", err)
	}

	up.hardTerminate <- up.Sy.HardTerminate()

	return nil
}

func (up *upContext) sync(ctx context.Context) error {
	if err := up.startSyncthing(ctx); err != nil {
		return err
	}

	start := time.Now()
	if err := config.UpdateStateFile(up.Dev.Name, up.Dev.Namespace, config.Synchronizing); err != nil {
		return err
	}

	startSyncFiles := time.Now()
	if err := up.synchronizeFiles(ctx); err != nil {
		return err
	}
	up.analyticsMeta.ContextSync(time.Since(startSyncFiles))

	msg := "Files synchronized"
	if up.Dev.IsHybridModeEnabled() {
		msg = "Reverse tunnel configured"
	}
	oktetoLog.Success(msg)

	elapsed := time.Since(start)
	up.analyticsMeta.InitialSyncDuration(elapsed)
	maxDuration := 1 * time.Minute
	if time.Duration(elapsed.Minutes()) > maxDuration {
		elapsedString := elapsed.String()
		oktetoLog.Warning(`File synchronization took %s
    Consider to update your '.stignore' to optimize the file synchronization
    More information is available here: https://okteto.com/docs/reference/file-synchronization/`, elapsedString)
	}

	up.Sy.Type = "sendreceive"
	up.Sy.IgnoreDelete = false
	if err := up.Sy.UpdateConfig(); err != nil {
		return err
	}

	go up.Sy.Monitor(ctx, up.Disconnect)
	go up.Sy.MonitorStatus(ctx, up.Disconnect)
	oktetoLog.Infof("restarting syncthing to update sync mode to sendreceive")
	return up.Sy.Restart(ctx)
}

func (up *upContext) startSyncthing(ctx context.Context) error {
	if !up.Dev.IsHybridModeEnabled() {
		oktetoLog.Spinner("Starting the file synchronization service...")
		oktetoLog.StartSpinner()
		defer oktetoLog.StopSpinner()
	}

	if err := config.UpdateStateFile(up.Dev.Name, up.Dev.Namespace, config.StartingSync); err != nil {
		return err
	}

	if err := up.Sy.Run(); err != nil {
		return err
	}

	if err := up.Sy.WaitForPing(ctx, true); err != nil {
		return err
	}

	if err := up.Sy.WaitForPing(ctx, false); err != nil {
		oktetoLog.Infof("failed to ping syncthing: %s", err.Error())
		if oktetoErrors.IsTransient(err) {
			return err
		}
		return up.checkOktetoStartError(ctx, "Failed to connect to the synchronization service")
	}

	if !up.Dev.IsHybridModeEnabled() {
		oktetoLog.Spinner("Scanning file system...")
	}
	startLocalScan := time.Now()
	if err := up.Sy.WaitForScanning(ctx, true); err != nil {
		return err
	}
	up.analyticsMeta.LocalFolderScan(time.Since(startLocalScan))

	if err := up.Sy.WaitForScanning(ctx, false); err != nil {
		return err
	}

	return up.Sy.WaitForConnected(ctx)
}

func (up *upContext) synchronizeFiles(ctx context.Context) error {
	if !up.Dev.IsHybridModeEnabled() {
		oktetoLog.Spinner("Synchronizing your files...")
		oktetoLog.StartSpinner()
		defer oktetoLog.StopSpinner()
	}

	progressBar := utils.NewSyncthingProgressBar(defaultProgressBarWidth)
	defer progressBar.Finish()

	quit := make(chan bool)

	go func() {
		for {
			select {
			case <-quit:
				return
			case <-time.NewTicker(1 * time.Second).C:
				inSynchronizationFile := up.Sy.GetInSynchronizationFile(ctx)
				if inSynchronizationFile != "" && oktetoLog.GetOutputFormat() != oktetoLog.PlainFormat {
					oktetoLog.StopSpinner()
					progressBar.UpdateItemInSync(inSynchronizationFile)
				}
			}
		}
	}()

	reporter := make(chan float64)
	go func() {
		for c := range reporter {
			value := int64(c)
			if value > 0 && value < 100 {
				if oktetoLog.GetOutputFormat() == oktetoLog.PlainFormat {
					oktetoLog.Spinner(fmt.Sprintf("Synchronizing your files [%d]...", value))
				} else {
					oktetoLog.StopSpinner()
					progressBar.SetCurrent(value)
				}
			}
		}
		quit <- true
	}()

	if err := up.Sy.WaitForCompletion(ctx, reporter); err != nil {
		up.analyticsMeta.ErrSync()
		switch err {
		case oktetoErrors.ErrLostSyncthing:
			up.analyticsMeta.ErrSyncLostSyncthing()
			return err
		case oktetoErrors.ErrInsufficientSpace:
			up.analyticsMeta.ErrSyncInsufficientSpace()
			return up.getInsufficientSpaceError(err)
		case oktetoErrors.ErrNeedsResetSyncError:
			up.analyticsMeta.ErrSyncResetDatabase()
			return oktetoErrors.UserError{
				E:    fmt.Errorf("the synchronization service state is inconsistent"),
				Hint: `Try running 'okteto up --reset' to reset the synchronization service`,
			}
		default:
			return oktetoErrors.UserError{
				E: err,
				Hint: fmt.Sprintf(`Help us improve okteto by filing an issue in https://github.com/okteto/okteto/issues/new.
    Please include the file generated by 'okteto doctor' if possible.
    Then, try to run '%s' + 'okteto up' again`, utils.GetDownCommand(up.Options.ManifestPathFlag)),
			}
		}
	}

	progressBar.SetCurrent(totalProgressValue)

	return nil
}

func (up *upContext) getSyncTempDir() (string, error) {
	return afero.TempDir(up.Fs, "", "")
}
