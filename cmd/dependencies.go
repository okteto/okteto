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
	"time"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/syncthing"
)

func downloadSyncthing() error {
	t := time.NewTicker(1 * time.Second)
	for i := 0; i < 3; i++ {
		err := syncthing.Install(defaultProgressBar)
		if err == nil {
			return nil
		}

		if i == 2 {
			return err
		}

		log.Infof("failed to download syncthing, retrying: %s", err)
		<-t.C
	}

	return fmt.Errorf("failed to download syncthing")
}
