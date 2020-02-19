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

package stack

import (
	"helm.sh/helm/v3/pkg/action"
)

const (
	// HelmDriver default helm driver
	HelmDriver = "secrets"
)

//ExistRelease returns if a release exists
func ExistRelease(actionConfig *action.Configuration, name string) (bool, error) {
	c := action.NewList(actionConfig)
	c.AllNamespaces = false
	results, err := c.Run()
	if err != nil {
		return false, err
	}
	for _, release := range results {
		if release.Name == name {
			return true, nil
		}
	}
	return false, nil
}
