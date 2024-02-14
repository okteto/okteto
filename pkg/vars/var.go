// Copyright 2024 The Okteto Authors
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

package vars

import (
	"fmt"
)

// Var represents a manifest variable
type Var struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
}

func (v *Var) String() string {
	return fmt.Sprintf("%s=%s", v.Name, v.Value)
}

//// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
//func (v *Var) UnmarshalYAML(unmarshal func(interface{}) error) error {
//	var raw string
//	err := unmarshal(&raw)
//	if err != nil {
//		return err
//	}
//
//	maxVarParts := 2
//	parts := strings.SplitN(raw, "=", maxVarParts)
//	v.Name = parts[0]
//	if len(parts) == maxVarParts {
//		v.Value = parts[1]
//		return nil
//	}
//
//	return nil
//}
//
//// MarshalYAML Implements the marshaler interface of the yaml pkg.
//func (v Var) MarshalYAML() (interface{}, error) {
//	return v.Name + "=" + v.Value, nil
//}
