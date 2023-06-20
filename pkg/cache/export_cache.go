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

// MarshalYAML implements the marshaller interface of the yaml pkg.
func (ec *ExportCache) MarshalYAML() (interface{}, error) {
	if len(*ec) == 1 {
		return (*ec)[0], nil
	}

	return ec, nil
}
