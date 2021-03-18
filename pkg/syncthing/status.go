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

package syncthing

import (
	"fmt"
)

const (
	// MinFileSizeToShowProgressBar is the mininmun size in bytes that a file must have to show its
	MinFileSizeToShowProgressBar = 5242880

	// MinSizeToShowProgressBar is the mininmun size in bytes that a directory must have to show its
	MinDirectorySizeToShowProgressBar = 52428800
)

type FolderStatus struct {
	LastDownloadEvent int
	LastFinishedEvent int
	Items             []*ItemInfo
}

type ItemInfo struct {
	Name       string
	Size       int64
	Proccessed int64
	ToShow     bool
	Children   []*ItemInfo
}

func (folder *FolderStatus) ToShowMap() map[string]int64 {
	progression := make(map[string]int64)
	for _, item := range folder.Items {
		if item.ToShow {
			if item.Proccessed > 0 {
				progression[item.Name] = item.Proccessed
			}
			continue
		}
		for _, child := range item.Children {
			for file, progress := range child.GetShowMap(item.Name) {
				progression[file] = progress
			}
		}
	}
	return progression
}

func (item *ItemInfo) GetShowMap(directory string) map[string]int64 {
	progression := make(map[string]int64)
	directory = fmt.Sprintf("%s/%s", directory, item.Name)
	for _, child := range item.Children {
		childName := fmt.Sprintf("%s/%s", directory, child.Name)
		if child.ToShow {
			progression[childName] = child.Proccessed
			return progression
		}
		for k, v := range child.GetShowMap(directory) {
			progression[k] = v
		}
	}
	return progression
}

func (folder *FolderStatus) updateProccessed() {
	for _, item := range folder.Items {
		item.Proccessed = item.CalculateProccessed()
	}
}

func (item *ItemInfo) CalculateProccessed() int64 {
	var proccess int64
	if len(item.Children) > 0 {
		for _, child := range item.Children {
			child.Proccessed = child.CalculateProccessed()
			proccess += child.Proccessed
		}
		item.Proccessed = proccess
	}
	return item.Proccessed
}
