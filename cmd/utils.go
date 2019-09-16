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

func askYesNo(q string) bool {
	var answer string
	for {
		fmt.Print(q)
		fmt.Scanln(&answer)
		if answer == "y" || answer == "n" {
			break
		}

		log.Fail("input must be 'y' or 'n'")
	}

	return answer == "y"
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
