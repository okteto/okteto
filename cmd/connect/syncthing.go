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
	"context"
	"fmt"
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/syncthing"
)

func (c *connectContext) initializeSyncthing() error {
	sy, err := syncthing.New(c.Dev, c.Namespace, c.Fs)
	if err != nil {
		return err
	}
	c.Sy = sy

	oktetoLog.Infof("local syncthing initialized: gui -> %d, sync -> %d", c.Sy.LocalGUIPort, c.Sy.LocalPort)
	oktetoLog.Infof("remote syncthing initialized: gui -> %d, sync -> %d", c.Sy.RemoteGUIPort, c.Sy.RemotePort)

	if err := c.Sy.SaveConfig(c.Dev, c.Namespace); err != nil {
		oktetoLog.Infof("error saving syncthing object: %s", err)
	}

	c.hardTerminate <- c.Sy.HardTerminate()
	return nil
}

func (c *connectContext) sync(ctx context.Context) error {
	if err := c.startSyncthing(ctx); err != nil {
		return err
	}

	if err := config.UpdateStateFile(c.Dev.Name, c.Namespace, config.Synchronizing); err != nil {
		return err
	}

	if err := c.synchronizeFiles(ctx); err != nil {
		return err
	}

	c.Sy.Type = "sendreceive"
	c.Sy.IgnoreDelete = false
	if err := c.Sy.UpdateConfig(); err != nil {
		return err
	}

	go c.Sy.Monitor(ctx, c.Disconnect)
	go c.Sy.MonitorStatus(ctx, c.Disconnect)
	oktetoLog.Infof("restarting syncthing to update sync mode to sendreceive")
	return c.Sy.Restart(ctx)
}

func (c *connectContext) startSyncthing(ctx context.Context) error {
	oktetoLog.Spinner("Starting the file synchronization service...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	if err := config.UpdateStateFile(c.Dev.Name, c.Namespace, config.StartingSync); err != nil {
		return err
	}

	if err := c.Sy.Run(); err != nil {
		return err
	}

	if err := c.Sy.WaitForPing(ctx, true); err != nil {
		return err
	}

	if err := c.Sy.WaitForPing(ctx, false); err != nil {
		oktetoLog.Infof("failed to ping syncthing: %s", err.Error())
		if oktetoErrors.IsTransient(err) {
			return err
		}
		return fmt.Errorf("failed to connect to the synchronization service: %w", err)
	}

	oktetoLog.Spinner("Scanning file system...")

	if err := c.Sy.WaitForScanning(ctx, true); err != nil {
		return err
	}
	if err := c.Sy.WaitForScanning(ctx, false); err != nil {
		return err
	}
	return c.Sy.WaitForConnected(ctx)
}

func (c *connectContext) synchronizeFiles(ctx context.Context) error {
	oktetoLog.Spinner("Synchronizing your files...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	const progressBarWidth = 40
	progressBar := utils.NewSyncthingProgressBar(progressBarWidth)
	defer progressBar.Finish()

	quit := make(chan bool)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				if f := c.Sy.GetInSynchronizationFile(ctx); f != "" && oktetoLog.GetOutputFormat() != oktetoLog.PlainFormat {
					oktetoLog.StopSpinner()
					progressBar.UpdateItemInSync(f)
				}
			}
		}
	}()

	reporter := make(chan float64)
	go func() {
		for v := range reporter {
			val := int64(v)
			if val > 0 && val < 100 {
				if oktetoLog.GetOutputFormat() == oktetoLog.PlainFormat {
					oktetoLog.Spinner(fmt.Sprintf("Synchronizing your files [%d]...", val))
				} else {
					oktetoLog.StopSpinner()
					progressBar.SetCurrent(val)
				}
			}
		}
		quit <- true
	}()

	if err := c.Sy.WaitForCompletion(ctx, reporter); err != nil {
		switch err {
		case oktetoErrors.ErrLostSyncthing:
			return err
		case oktetoErrors.ErrInsufficientSpace:
			return oktetoErrors.UserError{
				E:    err,
				Hint: "The synchronization service is running out of space. Enable persistent volumes in your okteto manifest.",
			}
		case oktetoErrors.ErrNeedsResetSyncError:
			return oktetoErrors.UserError{
				E:    fmt.Errorf("the synchronization service state is inconsistent"),
				Hint: "Try running 'okteto up --reset' to reset the synchronization service",
			}
		default:
			return err
		}
	}

	progressBar.SetCurrent(100)
	return nil
}
