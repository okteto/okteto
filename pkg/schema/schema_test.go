//	Copyright 2024 The Okteto Authors
//
// Copyright 2023|2024 The Okteto Authors
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
package schema

import (
	"testing"

	"github.com/okteto/okteto/pkg/model"
)

func difference(map1, map2 map[string][]string) map[string][]string {
	diff := make(map[string][]string)

	// Helper function to find the difference between two slices
	sliceDifference := func(slice1, slice2 []string) []string {
		m := make(map[string]bool)
		for _, s := range slice2 {
			m[s] = true
		}
		var diff []string
		for _, s := range slice1 {
			if !m[s] {
				diff = append(diff, s)
			}
		}
		return diff
	}

	// Check map1 against map2
	for key, slice1 := range map1 {
		if slice2, ok := map2[key]; ok {
			diffSlice := sliceDifference(slice1, slice2)
			if len(diffSlice) > 0 {
				diff[key] = diffSlice
			}
		} else {
			diff[key] = slice1
		}
	}

	// Check map2 against map1 for keys that are only in map2
	for key, slice2 := range map2 {
		if _, ok := map1[key]; !ok {
			diff[key] = slice2
		}
	}

	return diff
}

// TestDiffStructs ensures that the model.Manifest and schema.manifest structs are in sync
func TestDiffStructs(t *testing.T) {
	aKeys := model.GetStructKeys(model.Manifest{})
	bKeys := model.GetStructKeys(manifest{})

	differences := difference(aKeys, bKeys)

	if len(differences) > 0 {
		for key, value := range differences {
			t.Logf("Field %s differs: %v", key, value)
		}
		t.Errorf("model.Manifest and schema.manifest differ: %s", differences)
	}
}
