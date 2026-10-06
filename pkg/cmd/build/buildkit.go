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
	"github.com/okteto/okteto/pkg/build/buildkit"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type buildWriter struct{}

// statusDisplay shows the status of one solve until ch is closed. Errors sent to
// commandFail replace the BuildKit error returned by the solve.
type statusDisplay func(ch chan *client.SolveStatus, commandFail chan<- error) error

// SolveBuild runs a build and shows its progress in the given output mode
func SolveBuild(ctx context.Context, c *client.Client, opt *client.SolveOpt, progress string, ioCtrl *io.Controller, metadata *buildkit.BuildMetadata) error {
	return solve(ctx, c, opt, progress, metadata, func(ch chan *client.SolveStatus, commandFail chan<- error) error {
		return showBuildStatus(ch, commandFail, progress, ioCtrl)
	})
}

// solve runs the build, applies the Okteto log rules to its status, tracks the build
// context transfer into metadata and hands every status to display
func solve(ctx context.Context, c *client.Client, opt *client.SolveOpt, progress string, metadata *buildkit.BuildMetadata, display statusDisplay) error {
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
	contextTracker := newContextSyncTracker()
	ch := make(chan *client.SolveStatus)
	displayChannel := make(chan *client.SolveStatus)
	commandFailChannel := make(chan error, 1)

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		_, err = c.Solve(ctx, nil, *opt, ch)
		return errors.Wrap(err, "build failed")
	})

	eg.Go(func() error {
		relayStatus(ch, displayChannel, commandFailChannel, logFilter, contextTracker, metadata, progress)
		return nil
	})

	eg.Go(func() error {
		return display(displayChannel, commandFailChannel)
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

// relayStatus applies the Okteto log rules to every status of a solve, tracks the build
// context transfer into metadata and forwards each status to displayCh. It drains ch
// until the solve closes it (BuildKit closes it when Solve returns, also when the solve is
// cancelled) and then closes displayCh, so the display always ends. Returning early on
// cancellation would block BuildKit's status sender and hang the build.
func relayStatus(ch <-chan *client.SolveStatus, displayCh chan<- *client.SolveStatus, commandFail chan<- error, logFilter *BuildkitLogsFilter, contextTracker *contextSyncTracker, metadata *buildkit.BuildMetadata, progress string) {
	defer close(displayCh)
	for ss := range ch {
		logFilter.Run(ss, progress)
		contextTracker.Update(ss)
		if metadata != nil {
			metadata.BuildContextSize = contextTracker.SyncedSize()
			metadata.ContextTransferDuration = contextTracker.Duration()
		}
		displayCh <- ss
		if err := logFilter.GetError(ss); err != nil {
			commandFail <- err
		}
	}
}

// showBuildStatus shows the status of a single build: a live tty display whose plain
// output also goes to the Okteto log buffer, the deploy/destroy/test displayer, or plain
// output through ioCtrl
func showBuildStatus(ch chan *client.SolveStatus, commandFail chan<- error, progress string, ioCtrl *io.Controller) error {
	switch progress {
	case oktetoLog.TTYFormat:
		plainChannel := make(chan *client.SolveStatus)
		ttyChannel := make(chan *client.SolveStatus)
		go func() {
			for ss := range ch {
				plainChannel <- ss
				ttyChannel <- ss
			}
			close(plainChannel)
			close(ttyChannel)
		}()
		go showInLogBuffer(plainChannel)

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
		err := deployDisplayer(context.TODO(), ch, &types.BuildOptions{OutputMode: progress})
		commandFail <- err
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
		if _, err := d.UpdateFrom(context.TODO(), ch); err != nil {
			oktetoLog.Infof("could not display build status: %s", err)
		}
		return err
	}
}

// showInLogBuffer writes the plain output of a build into the Okteto log buffer
func showInLogBuffer(ch chan *client.SolveStatus) {
	w := &buildWriter{}
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
	if _, err := d.UpdateFrom(context.TODO(), ch); err != nil {
		oktetoLog.Infof("could not display build status: %s", err)
	}
}

func (*buildWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "%s", msg)
	return 0, nil
}
