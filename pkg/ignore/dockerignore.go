// Copyright 2024 The Okteto Authors
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

package ignore

import (
	"os"

	"github.com/moby/patternmatcher"
	"github.com/moby/patternmatcher/ignorefile"
)

type DockerIgnorer struct {
	patterns []string
}

func (di *DockerIgnorer) Ignore(filePath string) (bool, error) {
	return patternmatcher.Matches(filePath, di.patterns)
}

func NewDockerIgnorerFromFileOrNoop(dockerignoreFile string) *DockerIgnorer {
	di := &DockerIgnorer{patterns: []string{}}
	f, err := os.Open(dockerignoreFile)
	if err != nil {
		return di
	}
	defer f.Close()

	p, err := ignorefile.ReadAll(f)
	if err == nil {
		di.patterns = p
	}
	return di
}
