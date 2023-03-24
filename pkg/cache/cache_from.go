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

package cache

import (
	"fmt"

	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/registry"
)

// CacheFrom is a list of images to import cache from.
type CacheFrom []string

// UnmarshalYAML implements the Unmarshaler interface of the yaml pkg.
func (cf *CacheFrom) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var single string
	err := unmarshal(&single)
	if err == nil {
		*cf = CacheFrom{single}
		return nil
	}

	var multi []string
	err = unmarshal(&multi)
	if err == nil {
		*cf = multi
		return nil
	}

	return err
}

// MarshalYAML implements the marshaler interface of the yaml pkg.
func (cf CacheFrom) MarshalYAML() (interface{}, error) {
	if len(cf) == 1 {
		return cf[0], nil
	}

	return cf, nil
}

// Appends the default cache layers for a given image
func (cf *CacheFrom) AddDefaultPullCache(reg registry.OktetoRegistry, image string) {
	hasAccess, err := reg.HasGlobalPushAccess()
	if err != nil {
		oktetoLog.Infof("error trying to access globalPushAccess: %w", err)
	}

	_, repo := reg.ImageCtrl.GetRegistryAndRepo(image)
	imageName, _ := reg.ImageCtrl.GetRepoNameAndTag(repo)

	if hasAccess {
		globalCacheImage := fmt.Sprintf("%s/%s:cache", constants.GlobalRegistry, imageName)
		cf.addCacheFromImage(globalCacheImage)
	}

	devCacheImage := fmt.Sprintf("%s/%s:cache", constants.DevRegistry, imageName)
	cf.addCacheFromImage(devCacheImage)
}

// Append a cache image to the list if it's not already there
func (cf *CacheFrom) addCacheFromImage(imageName string) {
	if !cf.hasCacheFromImage(imageName) {
		*cf = append(*cf, imageName)
	}
}

// Check if a cache image is already in the list
func (cf *CacheFrom) hasCacheFromImage(imageName string) bool {
	for _, cacheFrom := range *cf {
		if cacheFrom == imageName {
			return true
		}
	}
	return false
}
