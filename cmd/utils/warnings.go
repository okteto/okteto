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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/okteto/okteto/pkg/log"
)

//GetWarningState returns the value associated to a given warning
func GetWarningState(path, name string) string {
	filePath := filepath.Join(path, name)
	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Infof("failed to read warning file '%s': %s", filePath, err)
		return ""
	}

	return string(bytes)
}

//SetWarningState sets the value associated to a given warning
func SetWarningState(path, name, value string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	filePath := filepath.Join(path, name)

	return ioutil.WriteFile(filePath, []byte(value), 0644)
}
