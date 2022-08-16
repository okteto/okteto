// Copyright 2022 The Okteto Authors
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

package manifest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
)

const (
	noneComposeOption = "None of the above"

	noMoreDockerfileOption = "No, thanks!"
	dockerfileName         = "Dockerfile"
)

func selectComposeFile(paths []string) (string, error) {
	if len(paths) == 1 {
		return paths[0], nil
	}

	paths = append(paths, noneComposeOption)
	selection, err := utils.AskForOptions(paths, "Select the compose to use:")
	if err != nil || selection == noneComposeOption {
		return "", err
	}

	return selection, nil
}

func selectDockerfiles(ctx context.Context, cwd string) ([]string, error) {
	dockerfiles := []string{}
	files, err := os.ReadDir(cwd)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if f.IsDir() {
			path := filepath.Join(cwd, f.Name())
			files, err := os.ReadDir(f.Name())
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				if f.IsDir() {
					continue
				}
				if strings.HasPrefix(f.Name(), dockerfileName) {
					dockerPath, err := filepath.Rel(cwd, filepath.Join(path, f.Name()))
					if err != nil {
						return nil, err
					}
					dockerfiles = append(dockerfiles, dockerPath)
				}
			}
		}
		if strings.HasPrefix(filepath.Base(f.Name()), dockerfileName) {
			dockerfiles = append(dockerfiles, f.Name())
		}
	}
	if err != nil {
		return nil, err
	}

	dockerfiles = append(dockerfiles, noMoreDockerfileOption)
	index := -1
	toConfigure := []string{}
	for index < 0 {
		if len(dockerfiles) == 1 {
			break
		}
		dockerfilesItems := []utils.SelectorItem{}
		for _, d := range dockerfiles {
			dockerfilesItems = append(dockerfilesItems, utils.SelectorItem{
				Name:   d,
				Label:  d,
				Enable: true,
			})
		}

		selection, _, err := utils.AskForOptionsOkteto(ctx, dockerfilesItems, "Do you need to build any of the following Dockerfiles as part of your development environment?", "")

		if err != nil {
			return nil, err
		}
		if selection == noMoreDockerfileOption {
			break
		} else {
			for idx, d := range dockerfiles {
				if d != selection {
					continue
				}
				dockerfiles = append(dockerfiles[:idx], dockerfiles[idx+1:]...)
				toConfigure = append(toConfigure, selection)
			}
		}
	}
	if err := validateDockerfileSelection(toConfigure); err != nil {
		return nil, err
	}

	return toConfigure, nil
}

func validateDockerfileSelection(dockerfiles []string) error {
	for idx, dockerfile := range dockerfiles {
		for otherIdx, otherDockerfile := range dockerfiles {
			if idx == otherIdx {
				continue
			}
			if filepath.Dir(dockerfile) == filepath.Dir(otherDockerfile) {
				return fmt.Errorf("can not configure two dockerfiles from the same directory")
			}
		}
	}
	return nil
}
