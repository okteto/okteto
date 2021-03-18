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

package utils

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/vbauerster/mpb/v6"
	mbp "github.com/vbauerster/mpb/v6"
	"github.com/vbauerster/mpb/v6/decor"
)

const (
	GroupWidth           = 40
	MainProgressBarWidth = 40
	FileProgressBarWidth = 30
	MaxNameChars         = 20
)

// ProgressBar tracks progress of the download
type ProgressBar struct {
	// lock everything below
	lock sync.Mutex
}

// SyncthingProgress tracks the progress of all the files syncthing
type SyncthingProgress struct {
	Group          *mbp.Progress
	MainBar        *mbp.Bar
	FileProgress   map[string]Progress
	waitGroup      *sync.WaitGroup
	LastItemInSync string
}

type Progress struct {
	Size  int64
	Bar   *mbp.Bar
	Start time.Time
}

// TrackProgress instantiates a new progress bar that will
// display the progress of stream until closed.
// total can be 0.
func (cpb *ProgressBar) TrackProgress(src string, currentSize, totalSize int64, stream io.ReadCloser) io.ReadCloser {
	cpb.lock.Lock()
	defer cpb.lock.Unlock()

	newPb := pb.New64(totalSize)
	newPb.Set("prefix", fmt.Sprintf("%s ", filepath.Base(src)))
	newPb.SetCurrent(currentSize)
	newPb.Start()
	reader := newPb.NewProxyReader(stream)

	return &readCloser{
		Reader: reader,
		close: func() error {
			cpb.lock.Lock()
			defer cpb.lock.Unlock()

			newPb.Finish()
			return nil
		},
	}
}

type readCloser struct {
	io.Reader
	close func() error
}

func (c *readCloser) Close() error { return c.close() }

// RenderProgressBar displays a progress bar
func RenderProgressBar(prefix string, current, scalingFactor float64) string {
	var sb strings.Builder
	_, _ = sb.WriteString(prefix)
	_, _ = sb.WriteString("[")

	scaledMax := int(100 * scalingFactor)
	scaledCurrent := int(current * scalingFactor)

	switch {
	case scaledCurrent == 0:
		_, _ = sb.WriteString(strings.Repeat("_", scaledMax))
	case scaledCurrent >= scaledMax:
		_, _ = sb.WriteString(strings.Repeat("-", scaledMax))
	default:
		_, _ = sb.WriteString(strings.Repeat("-", scaledCurrent-1))
		_, _ = sb.WriteString(">")
		_, _ = sb.WriteString(strings.Repeat("_", scaledMax-scaledCurrent))
	}

	_, _ = sb.WriteString("]")
	_, _ = sb.WriteString(fmt.Sprintf(" %3v%%", int(current)))
	return sb.String()
}

// New creates a new syncthing progress
func NewSyncthingProgressBar() *SyncthingProgress {
	var s SyncthingProgress
	s.waitGroup = new(sync.WaitGroup)
	s.Group = mbp.New(mbp.WithWidth(GroupWidth))
	s.FileProgress = make(map[string]Progress)
	s.MainBar = s.NewMainBar(MainProgressBarWidth)
	return &s
}

// NewBar creates a new bar to the group of progress bars
func (s *SyncthingProgress) NewMainBar(width int) *mbp.Bar {

	return s.Group.Add(100, nil,
		mpb.PrependDecorators(
			decor.OnComplete(decor.Spinner(nil, decor.WCSyncSpace), "Files synchronized"),
			decor.OnComplete(decor.Name(" Synchronizing: "), ""),
			decor.OnComplete(s.ItemStartedDecorator(), ""),
		),
		mpb.BarExtender(NewLineBarFiller(mpb.NewBarFiller("[->_]"))),
		mbp.BarWidth(width+FileProgressBarWidth-MaxNameChars),
		mpb.BarRemoveOnComplete(),
	)
}

// NewBar creates a new bar to the group of progress bars
func (s *SyncthingProgress) NewBar(name string, total int64, width int) *mbp.Bar {
	return s.Group.Add(total,
		// Bar style
		mpb.NewBarFiller("[->_]"),
		mpb.BarFillerTrim(),
		mpb.PrependDecorators(
			decor.Name(log.GreyString("   %s", s.GetNameDisplay(name)), decor.WC{W: len(s.GetNameDisplay(name)) + 1, C: decor.DidentRight}),
		),
		mpb.AppendDecorators(decor.NewPercentage("  %d")),
		mbp.BarWidth(width),
		mpb.BarRemoveOnComplete(),
	)
}

func (s *SyncthingProgress) UpdateLastItemInSync(lastItem string) {
	s.LastItemInSync = lastItem
}

// UpdateProgress updates all the progress bars in the pool
func (s *SyncthingProgress) UpdateProgress(syncthingItems map[string]*syncthing.FolderStatus) {
	for _, folderInfo := range syncthingItems {
		dirProgressionMap := folderInfo.ToShowMap()
		for file, value := range dirProgressionMap {
			if s.IsRegistered(file) {
				s.UpdateBar(file, value)
			} else {
				item := syncthing.GetItem(file, folderInfo.Items)
				totalSize := item.Size
				s.FileProgress[file] = Progress{Bar: s.NewBar(file, totalSize, FileProgressBarWidth), Size: totalSize}
				s.UpdateBar(file, value)
			}
		}

	}

}

// UpdateBar updates the progress bar to a certain value
func (s *SyncthingProgress) UpdateBar(name string, value int64) {
	s.FileProgress[name].Bar.SetCurrent(value)
}

// isRegistered checks if a bar name exists in the pool of progressBars
func (s *SyncthingProgress) IsRegistered(name string) bool {
	_, exists := s.FileProgress[syncthing.GetRootPath(name)]
	return exists
}

// GetNameDisplaye set the max num char of a file to a constant
func (s *SyncthingProgress) GetNameDisplay(name string) string {
	nameLength := len(name)
	if nameLength <= MaxNameChars {
		return fmt.Sprintf("%s%s", name, strings.Repeat(" ", MaxNameChars-nameLength))
	} else {
		displayName := name[:MaxNameChars-3]
		return fmt.Sprintf("%s...", displayName)
	}
}

func (s *SyncthingProgress) Finish() {
	for _, progressBar := range s.FileProgress {
		completed := progressBar.Size
		progressBar.Bar.SetCurrent(completed)
	}
	s.MainBar.SetCurrent(100)
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
	var msg string
	fn := func(s decor.Statistics) string {
		if !s.Completed {
			msg = sync.LastItemInSync
		}
		return msg
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
