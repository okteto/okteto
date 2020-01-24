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

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

func loadDev(devPath string) (*model.Dev, error) {
	if !fileExists(devPath) {
		if devPath == defaultManifest {
			if fileExists(secondaryManifest) {
				return loadDev(secondaryManifest)
			}
		}

		return nil, fmt.Errorf("'%s' does not exist. Generate it by executing 'okteto init'", devPath)
	}

	return model.Get(devPath)
}

func askYesNo(q string) (bool, error) {
	var answer string
	for {
		fmt.Print(q)
		if _, err := fmt.Scanln(&answer); err != nil {
			return false, err
		}

		if answer == "y" || answer == "n" {
			break
		}

		log.Fail("input must be 'y' or 'n'")
	}

	return answer == "y", nil
}

func fileExists(name string) bool {

	_, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false
	}

	if err != nil {
		log.Infof("Failed to check if %s exists: %s", name, err)
	}

	return true
}

func checkWatchesConfiguration() {
	if runtime.GOOS != "linux" {
		return
	}

	w := "/proc/sys/fs/inotify/max_user_watches"
	f, err := ioutil.ReadFile(w)
	if err != nil {
		log.Infof("Fail to read %s: %s", w, err)
		return
	}

	l := strings.TrimSuffix(string(f), "\n")
	c, err := strconv.Atoi(l)
	if err != nil {
		log.Infof("Fail to parse the value of  max_user_watches: %s", err)
		return
	}

	log.Debugf("max_user_watches = %d", c)

	if c <= 8192 {
		log.Yellow("The value of /proc/sys/fs/inotify/max_user_watches is too low. This can affect Okteto's file synchronization performance.")
		log.Yellow("We recommend you to raise it to at least 524288 to ensure proper performance.")
		fmt.Println()
	}
}
