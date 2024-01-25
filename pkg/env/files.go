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

package env

// Files is a list of environment files
type Files []string

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (envFiles *Files) UnmarshalYAML(unmarshal func(interface{}) error) error {
	result := make(Files, 0)
	var single string
	err := unmarshal(&single)
	if err != nil {
		var multi []string
		err := unmarshal(&multi)
		if err != nil {
			return err
		}
		result = multi
		*envFiles = result
		return nil
	}

	result = append(result, single)
	*envFiles = result
	return nil
}
