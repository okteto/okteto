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

package deployable

import (
	"fmt"
	"sync"

	"github.com/compose-spec/godotenv"
	"github.com/spf13/afero"
)

// envStepper is an environment reader utility that reads the environment from
// a temporary .env file and keeps a reference to it. It is meant to be called
// to read environment variables in a command list loop
type envStepper struct {
	fs       afero.Fs
	env      map[string]string
	filename string
	sync.RWMutex
}

func (es *envStepper) WithFS(fs afero.Fs) {
	es.fs = fs
}
func (es *envStepper) Step() ([]string, error) {
	data, err := afero.ReadFile(es.fs, es.filename)
	if err != nil {
		return nil, err
	}

	current, err := godotenv.Unmarshal(string(data))
	if err != nil {
		return nil, err
	}

	list := mapToEnvList(current)

	es.Lock()
	es.env = current
	es.Unlock()

	return list, nil
}

func (es *envStepper) Map() (env map[string]string) {
	es.RLock()
	env = es.env
	es.RUnlock()
	return
}

func NewEnvStepper(filename string) *envStepper {
	return &envStepper{
		RWMutex:  sync.RWMutex{},
		filename: filename,
		env:      make(map[string]string),
		fs:       afero.NewOsFs(),
	}
}

func mapToEnvList(env map[string]string) []string {
	list := make([]string, 0, len(env))
	for k, v := range env {
		list = append(list, fmt.Sprintf("%s=%s", k, v))
	}
	return list
}
