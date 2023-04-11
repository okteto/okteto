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
)

// ExportCache is a list of images that will be created to export the cache.
type ExportCache []string

// UnmarshalYAML implements the Unmarshaler interface of the yaml pkg.
func (ec *ExportCache) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var single string
	err := unmarshal(&single)
	if err == nil {
		*ec = ExportCache{single}
		return nil
	}

	var multi []string
	err = unmarshal(&multi)
	if err == nil {
		*ec = multi
		return nil
	}

	return err
}

// MarshalYAML implements the marshaler interface of the yaml pkg.
func (pec *ExportCache) MarshalYAML() (interface{}, error) {
	ec := *pec
	if len(ec) == 1 {
		return ec[0], nil
	}

	return ec, nil
}

// AddDefaultPushCache appends the default cache layers for a given image
func (pec *ExportCache) AddDefaultPushCache(reg oktetoRegistryInterface, image string) {
	_, imageRepo := reg.GetRegistryAndRepo(image)
	imageName, _ := reg.GetRepoNameAndTag(imageRepo)

	if reg.IsGlobalRegistry(image) {
		newCache := fmt.Sprintf("%s/%s:%s", constants.GlobalRegistry, imageName, defaultCacheTag)
		pec.add(newCache)
		return
	}
	newDevCache := fmt.Sprintf("%s/%s:%s", constants.DevRegistry, imageName, defaultCacheTag)
	pec.add(newDevCache)

}

// add appends the image to the list of export cache images
func (pec *ExportCache) add(image string) {
	for _, c := range *pec {
		if c == image {
			return
		}
	}
	*pec = append(*pec, image)
}
