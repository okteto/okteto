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

package utils

import (
	"fmt"
	"io"

	"github.com/vbauerster/mpb/v6"
	decor "github.com/vbauerster/mpb/v6/decor"
)

// SyncthingProgress tracks the progress of all the files syncthing
type SyncthingProgress struct {
	progressContainer *mpb.Progress
	progressBar       *mpb.Bar
	itemInSync        string
}

// NewSyncthingProgressBar creates a new syncthing progress
func NewSyncthingProgressBar(width int) *SyncthingProgress {
	return &SyncthingProgress{
		progressContainer: mpb.New(mpb.WithWidth(width)),
	}
}

func (s *SyncthingProgress) initProgressBar() {
	s.progressBar = s.progressContainer.Add(
		100,
		nil,
		mpb.PrependDecorators(
			decor.OnComplete(decor.Spinner(nil, decor.WCSyncSpace), "Files synchronized"),
			decor.OnComplete(decor.Name(" "), ""),
			decor.OnComplete(s.ItemStartedDecorator(), ""),
		),
		mpb.BarExtender(NewLineBarFiller(mpb.NewBarFiller("[->_]"))),
		mpb.BarRemoveOnComplete(),
	)
}

// UpdateItemInSync updates the item in sync
func (s *SyncthingProgress) UpdateItemInSync(lastItem string) {
	s.itemInSync = lastItem
	if s.progressBar == nil {
		s.initProgressBar()
	}
}

// SetCurrent sets current progress of the syncthing progress bar
func (s *SyncthingProgress) SetCurrent(v int64) {
	if s.progressBar == nil {
		s.initProgressBar()
	}
	s.progressBar.SetCurrent(v)
}

// Finish finishes the progress bar
func (s *SyncthingProgress) Finish() {
	if s.progressBar != nil {
		s.progressBar.SetCurrent(100)
	}
	s.progressContainer.Wait()
}

func NewLineBarFiller(filler mpb.BarFiller) mpb.BarFiller {
	return mpb.BarFillerFunc(func(w io.Writer, reqWidth int, st decor.Statistics) {
		w.Write([]byte("   "))
		filler.Fill(w, reqWidth, st)
		percentage := Percentage(st.Total, st.Current, 100)
		afterBarText := fmt.Sprintf(" %d%%\n", int(percentage))
		w.Write([]byte(afterBarText))
	})
}

func (sync *SyncthingProgress) ItemStartedDecorator(wcc ...decor.WC) decor.Decorator {
	fn := func(s decor.Statistics) string {
		if sync.itemInSync != "" {
			return fmt.Sprintf("Synchronizing %s...", sync.itemInSync)
		}
		return "Synchronizing your files..."
	}
	return decor.Any(fn, wcc...)
}

func Percentage(total, current int64, width int) float64 {
	if total <= 0 {
		return 0
	}
	if current >= total {
		return float64(width)
	}
	return float64(int64(width)*current) / float64(total)
}
