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

package up

import (
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
)

func checkLocalWatchesConfiguration() {
	if runtime.GOOS != "linux" {
		return
	}

	warningFolder := filepath.Join(config.GetOktetoHome(), ".warnings")
	if utils.GetWarningState(warningFolder, "localwatcher") != "" {
		return
	}

	w := "/proc/sys/fs/inotify/max_user_watches"
	f, err := ioutil.ReadFile(w)
	if err != nil {
		log.Infof("Fail to read %s: %s", w, err)
		return
	}

	if isWatchesConfigurationTooLow(string(f)) {
		log.Yellow("The value of /proc/sys/fs/inotify/max_user_watches is too low.")
		log.Yellow("This can affect Okteto's file synchronization performance.")
		log.Yellow("We recommend you to raise it to at least 524288 to ensure proper performance.")
		if err := utils.SetWarningState(warningFolder, "localwatcher", "true"); err != nil {
			log.Infof("failed to set warning localwatcher state: %s", err.Error())
		}
	}
}

func isWatchesConfigurationTooLow(value string) bool {
	value = strings.TrimSuffix(string(value), "\n")
	if value == "" {
		log.Infof("max_user_watches is empty '%s'", value)
		return false
	}

	c, err := strconv.Atoi(value)
	if err != nil {
		log.Infof("failed to parse the value of max_user_watches: %s", err)
		return false
	}

	return c <= 8192
}
