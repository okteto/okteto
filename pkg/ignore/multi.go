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
package ignore

// MultiIgnorer is an Ignorer that combines multiple Ignorers.
// Ignore returns true if any of the Ignorers returns true.
type MultiIgnorer struct {
	ignorers []Ignorer
}

func (mi *MultiIgnorer) Ignore(filePath string) (bool, error) {
	for _, i := range mi.ignorers {
		if i == nil {
			continue
		}
		ignore, err := i.Ignore(filePath)
		if err != nil {
			return false, err
		}
		if ignore {
			return true, nil
		}
	}
	return false, nil
}

func NewMultiIgnorer(ignorers ...Ignorer) *MultiIgnorer {
	return &MultiIgnorer{ignorers: ignorers}
}
