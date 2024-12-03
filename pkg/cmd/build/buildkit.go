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

package build

import (
	"context"
	"os"
	"strings"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progressui"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type buildWriter struct{}

func SolveBuild(ctx context.Context, c *client.Client, opt *client.SolveOpt, progress string, ioCtrl *io.Controller) error {
	logFilterRules := []LogRule{
		{
			condition:   BuildKitMissingCacheCondition,
			transformer: BuildKitMissingCacheTransformer,
		},
	}
	errorRules := []ErrorRule{
		{
			condition: BuildKitFrontendNotFoundErr,
		},
	}
	logFilter := NewBuildKitLogsFilter(logFilterRules, errorRules)
	ch := make(chan *client.SolveStatus)
	ttyChannel := make(chan *client.SolveStatus)
	plainChannel := make(chan *client.SolveStatus)
	commandFailChannel := make(chan error, 1)

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		_, err = c.Solve(ctx, nil, *opt, ch)
		return errors.Wrap(err, "build failed")
	})

	eg.Go(func() error {
		done := false
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ss, ok := <-ch:
				if ok {
					logFilter.Run(ss, progress)
					plainChannel <- ss
					if progress == oktetoLog.TTYFormat {
						ttyChannel <- ss
					}
					if err := logFilter.GetError(ss); err != nil {
						commandFailChannel <- err
					}
				} else {
					done = true
				}

			}
			if done {
				close(plainChannel)
				if progress == oktetoLog.TTYFormat {
					close(ttyChannel)
				}
				break
			}
		}
		return nil
	})

	eg.Go(func() error {

		w := &buildWriter{}
		switch progress {
		case oktetoLog.TTYFormat:
			go func() {
				// We use the plain channel to store the logs into a buffer and then show them in the UI
				d, err := progressui.NewDisplay(w, progressui.PlainMode)
				if err != nil {
					// If an error occurs while attempting to create the tty display,
					// fallback to using plain mode on stdout (in contrast to stderr).
					d, err = progressui.NewDisplay(w, progressui.PlainMode)
					if err != nil {
						oktetoLog.Infof("could not display build status: %s", err)
						return
					}
				}
				// not using shared context to not disrupt display but let is finish reporting errors
				if _, err := d.UpdateFrom(context.TODO(), plainChannel); err != nil {
					oktetoLog.Infof("could not display build status: %s", err)
				}
			}()
			// not using shared context to not disrupt display but let it finish reporting errors
			// We need to wait until the tty channel is closed to avoid writing to stdout while the tty is being used
			d, err := progressui.NewDisplay(os.Stdout, progressui.TtyMode)
			if err != nil {
				// If an error occurs while attempting to create the tty display,
				// fallback to using plain mode on stdout (in contrast to stderr).
				d, err = progressui.NewDisplay(os.Stdout, progressui.PlainMode)
				if err != nil {
					oktetoLog.Infof("could not display build status: %s", err)
					return err
				}
			}
			// not using shared context to not disrupt display but let is finish reporting errors
			if _, err := d.UpdateFrom(context.TODO(), ttyChannel); err != nil {
				oktetoLog.Infof("could not display build status: %s", err)
			}
			return err
		case DeployOutputModeOnBuild, DestroyOutputModeOnBuild, TestOutputModeOnBuild:
			err := deployDisplayer(context.TODO(), plainChannel, &types.BuildOptions{OutputMode: progress})
			commandFailChannel <- err
			return err
		default:
			// not using shared context to not disrupt display but let it finish reporting errors
			d, err := progressui.NewDisplay(ioCtrl.Out(), progressui.PlainMode)
			if err != nil {
				// If an error occurs while attempting to create the tty display,
				// fallback to using plain mode on stdout (in contrast to stderr).
				d, err = progressui.NewDisplay(os.Stdout, progressui.PlainMode)
				if err != nil {
					oktetoLog.Infof("could not display build status: %s", err)
					return err
				}
			}
			// not using shared context to not disrupt display but let is finish reporting errors
			if _, err := d.UpdateFrom(context.TODO(), plainChannel); err != nil {
				oktetoLog.Infof("could not display build status: %s", err)
			}
			return err
		}
	})

	err := eg.Wait()
	// If the command failed, we want to return the error from the command instead of the buildkit error
	if err != nil {
		select {
		case commandErr := <-commandFailChannel:
			if commandErr != nil {
				return commandErr
			}
			return err
		default:
			return err
		}
	}
	return nil
}

func (*buildWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, msg)
	return 0, nil
}
