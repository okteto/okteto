// Copyright 2023 The Okteto Authors
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

package repository

import (
	"context"
	"fmt"
)

type oktetoRemoteRepoController struct {
	gitCommit string
}

func newOktetoRemoteRepoController(localCommit string) oktetoRemoteRepoController {
	return oktetoRemoteRepoController{
		gitCommit: localCommit,
	}
}

func (or oktetoRemoteRepoController) isClean(_ context.Context) (bool, error) {
	return or.gitCommit != "", nil
}

func (or oktetoRemoteRepoController) getSHA() (string, error) {
	return or.gitCommit, nil
}

func (or oktetoRemoteRepoController) getTreeSHA(string) (string, error) {
	return "", fmt.Errorf("not-implemented")
}
