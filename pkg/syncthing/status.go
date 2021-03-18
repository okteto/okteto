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
	"context"
	"fmt"
	"strings"
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

func (s *Syncthing) GetTreeByNeed(ctx context.Context, folder *Folder) error {
	neededFiles, err := s.GetNeeded(ctx, folder)
	if err != nil {
		return err
	}
	s.Status[GetFolderName(folder)].Items = createTree(neededFiles)
	updateFolderSizes(s.Status[GetFolderName(folder)].Items)
	return nil
}

func (s *Syncthing) SetLastItemFinished(ctx context.Context, folder *Folder) error {
	finishedItems, err := s.GetFinished(ctx, folder, 0)
	if err != nil {
		return err
	}
	if len(finishedItems) > 0 {
		lastFinishedItem := finishedItems[len(finishedItems)-1]
		s.Status[GetFolderName(folder)].LastFinishedEvent = lastFinishedItem.Id
	}
	return nil
}

func (s *Syncthing) GetFilesThatShouldBeIgnored() string {
	toIgnoreFiles := make([]string, 0)
	for _, folderInfo := range s.Status {
		dirProgressionMap := folderInfo.ToShowMap()
		for name := range dirProgressionMap {

			toIgnoreFiles = append(toIgnoreFiles, fmt.Sprintf("- %s", name))
		}
	}
	return strings.Join(toIgnoreFiles, "/n    ")
}

func createTree(neededFiles map[string]int) []*ItemInfo {
	directoryTree := make([]*ItemInfo, 0)
	for fileName, fileSize := range neededFiles {
		if isInDir(fileName) {
			directoryTree = addFileInDirectory(fileName, fileSize, directoryTree)
		} else {
			file := &ItemInfo{Name: fileName, Size: int64(fileSize)}
			if file.Size > MinFileSizeToShowProgressBar {
				file.ToShow = true
			}
			directoryTree = append(directoryTree, file)
		}

	}
	return directoryTree
}

func isInDir(filePath string) bool {
	return strings.Contains(filePath, "/")
}

func addFileInDirectory(name string, size int, actualTreeStatus []*ItemInfo) []*ItemInfo {
	splittedName := strings.Split(name, "/")
	folders := splittedName[:len(splittedName)-1]
	var previousFolder *ItemInfo
	for _, folderName := range folders {
		var folder *ItemInfo
		if previousFolder == nil {
			folder = getFolderIfExists(folderName, actualTreeStatus)
		} else {
			folder = getFolderIfExists(folderName, previousFolder.Children)
		}
		if folder == nil {
			folder = &ItemInfo{Name: folderName, Children: make([]*ItemInfo, 0)}
			if previousFolder == nil {
				actualTreeStatus = append(actualTreeStatus, folder)
			} else {
				previousFolder.Children = append(previousFolder.Children, folder)
			}
		}
		previousFolder = folder
	}

	fileName := splittedName[len(splittedName)-1]
	file := &ItemInfo{Name: fileName, Size: int64(size)}
	if file.Size > MinFileSizeToShowProgressBar {
		file.ToShow = true
	}
	if previousFolder != nil {
		previousFolder.Children = append(previousFolder.Children, file)
	} else {
		actualTreeStatus = append(actualTreeStatus, file)
	}
	return actualTreeStatus
}

func getFolderIfExists(folderName string, actualFolder []*ItemInfo) *ItemInfo {
	for _, file := range actualFolder {
		if file.Name == folderName {
			return file
		}
	}
	return nil
}

func createFolder(folderName string, actualFolder []*ItemInfo) *ItemInfo {
	newFolder := &ItemInfo{Name: folderName, Children: make([]*ItemInfo, 0)}
	actualFolder = append(actualFolder, newFolder)
	return newFolder
}

func addFileToFolder(fileName string, size int, actualFolder []*ItemInfo) []*ItemInfo {
	file := &ItemInfo{Name: fileName, Size: int64(size)}
	if file.Size > MinFileSizeToShowProgressBar {
		file.ToShow = true
	}
	actualFolder = append(actualFolder, file)
	return actualFolder
}

func updateFolderSizes(actualFolder []*ItemInfo) {
	for _, item := range actualFolder {
		if len(item.Children) > 0 {
			for _, child := range item.Children {
				child.Size = child.CalculateSize()
			}
			item.Size = item.CalculateSize()
			if item.IsDirToShow() {
				item.ToShow = true
				item.HideChildren()
			}
		}
	}
}

func (item *ItemInfo) CalculateSize() int64 {
	var size int64
	if len(item.Children) > 0 {
		for _, child := range item.Children {
			size += child.CalculateSize()
		}
	} else {
		size += item.Size
	}

	return size
}

func GetItem(fileName string, items []*ItemInfo) *ItemInfo {
	var emptyItem *ItemInfo
	fileByFolder := strings.Split(fileName, "/")
	root := fileByFolder[0]
	toFind := strings.Join(fileByFolder[1:], "/")
	for _, item := range items {
		name := item.Name
		if name == root {
			if toFind == "" {
				return item
			}
			if len(item.Children) > 0 {
				itemPartial := GetItem(toFind, item.Children)
				if itemPartial != nil {
					if itemPartial.IsDirToShow() {
						itemPartial.ToShow = true
						itemPartial.HideChildren()
						return itemPartial
					}
					return emptyItem
				}
			}
			return item
		}

	}
	return emptyItem
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

func (folder *FolderStatus) setProccess(itemProccess map[string]int64) {
	items := &folder.Items
	for name, proccess := range itemProccess {
		item := GetItem(name, *items)
		if item != nil {
			item.Proccessed = proccess
		}
	}
	folder.updateProccessed()
}

func (item *ItemInfo) HideChildren() {
	for _, child := range item.Children {
		child.ToShow = false
	}
}

func (item *ItemInfo) IsDirToShow() bool {
	var toShowChildren int

	for _, child := range item.Children {
		if child.ToShow {
			toShowChildren++
		}
		if toShowChildren == 2 {
			return true
		}
	}
	if item.Size > MinDirectorySizeToShowProgressBar {
		return true
	}
	return false
}

func (folder *FolderStatus) setFinishedItems(finishedItems []string) {
	items := folder.Items
	for _, finishedItem := range finishedItems {
		item := GetItem(finishedItem, items)
		if item != nil {
			item.Proccessed = item.Size
		}
	}
	folder.updateProccessed()
}
