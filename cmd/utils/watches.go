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
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/okteto/okteto/pkg/log"
)

//FileExists checks if a file exists
func FileExists(name string) bool {

	_, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false
	}

	if err != nil {
		log.Infof("Failed to check if %s exists: %s", name, err)
	}

	return true
}

//CheckLocalWatchesConfiguration shows a warning if local watcches are too low
func CheckLocalWatchesConfiguration() {
	if runtime.GOOS != "linux" {
		return
	}

	w := "/proc/sys/fs/inotify/max_user_watches"
	f, err := ioutil.ReadFile(w)
	if err != nil {
		log.Infof("Fail to read %s: %s", w, err)
		return
	}

	if IsWatchesConfigurationTooLow(string(f)) {
		log.Yellow("The value of /proc/sys/fs/inotify/max_user_watches is too low.")
		log.Yellow("This can affect Okteto's file synchronization performance.")
		log.Yellow("We recommend you to raise it to at least 524288 to ensure proper performance.")
		fmt.Println()
	}
}

//IsWatchesConfigurationTooLow returns if watches configuration is too low
func IsWatchesConfigurationTooLow(value string) bool {
	value = strings.TrimSuffix(string(value), "\n")
	c, err := strconv.Atoi(value)
	if err != nil {
		log.Infof("Fail to parse the value of max_user_watches: %s", err)
		return false
	}
	log.Debugf("max_user_watches = %d", c)
	return c <= 8192
}
