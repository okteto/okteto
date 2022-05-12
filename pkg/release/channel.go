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

package release

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/config"
)

const downloadsUrl = "https://downloads.okteto.com"

const releaseChannelDev = "dev"
const releaseChannelBeta = "beta"
const releaseChannelStable = "stable"

// GetReleaseChannel returns the release channel for the current installation.
// It does so by reading the $HOME/.okteto/channel file
// If the release channel is empty it is assumed that we are on the stable channel
func GetReleaseChannel() (string, error) {
	filename := fmt.Sprintf("%s/channel", config.GetOktetoHome())
	b, err := os.ReadFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	s := strings.TrimSuffix(string(b), "\n")
	if s == "" {
		s = releaseChannelStable
	}
	return s, nil
}

// UpdateReleaseChannel updates the release channel for the current installation
// It does so by overriding the contents of the $HOME/.okteto/channel file
func UpdateReleaseChannel(v string) error {
	filename := fmt.Sprintf("%s/channel", config.GetOktetoHome())
	switch {
	case v == "":
		return os.Remove(filename)
	case v == releaseChannelBeta || v == releaseChannelDev || v == releaseChannelStable:
		if err := os.WriteFile(filename, []byte(v+"\n"), 0644); err != nil {
			return err
		}
		return nil
	default:
		return errors.New("invalid channel")
	}
}
