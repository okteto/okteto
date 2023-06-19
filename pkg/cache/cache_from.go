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

// CacheFrom is a list of images to import cache from.
type CacheFrom []string

type oktetoRegistryInterface interface {
	HasGlobalPushAccess() (bool, error)
	IsGlobalRegistry(image string) bool

	GetRegistryAndRepo(image string) (string, string)
	GetRepoNameAndTag(repo string) (string, string)
}

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
func (cf *CacheFrom) MarshalYAML() (interface{}, error) {
	if len(*cf) == 1 {
		return (*cf)[0], nil
	}

	return cf, nil
}
