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

package status

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/syncthing"
)

//Run runs the "okteto status" sequence
func Run(ctx context.Context, dev *model.Dev, sy *syncthing.Syncthing) (float64, error) {
	progressLocal, err := sy.GetStatus(ctx, dev, true)
	if err != nil {
		return 0, fmt.Errorf("error accessing local syncthing status: %s", err)
	}
	progressRemote, err := sy.GetStatus(ctx, dev, false)
	if err != nil {
		return 0, fmt.Errorf("error accessing remote syncthing status: %s", err)
	}
	progress := (progressLocal + progressRemote) / 2
	return progress, nil
}
