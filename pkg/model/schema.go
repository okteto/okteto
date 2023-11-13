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

package model

import (
	"reflect"
	"strings"
)

func mergeAndSortUnique(slice1, slice2 []string) []string {
	uniqueMap := make(map[string]struct{})
	var result []string

	for _, str := range slice1 {
		if _, exists := uniqueMap[str]; !exists {
			uniqueMap[str] = struct{}{}
			result = append(result, str)
		}
	}

	for _, str := range slice2 {
		if _, exists := uniqueMap[str]; !exists {
			uniqueMap[str] = struct{}{}
			result = append(result, str)
		}
	}

	return result
}

// getStructKeys recursively goes through a given struct and returns a map of struct names to their fields
func getStructKeys(t interface{}) map[string][]string {
	result := make(map[string][]string)
	typ := reflect.TypeOf(t)

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// Handle map type at top level
	if typ.Kind() == reflect.Map {
		// For each key in the map type, check if the value type is a struct and process accordingly
		mapValueType := typ.Elem()
		if mapValueType.Kind() == reflect.Ptr {
			mapValueType = mapValueType.Elem()
		}

		if mapValueType.Kind() == reflect.Struct {
			return getStructKeys(reflect.New(mapValueType).Interface())
		}
		return result
	}

	if typ.Kind() != reflect.Struct {
		return result
	}

	var structFullName string

	if typ.Name() == "" { // Anonymous struct
		structFullName = "_"
	} else {
		pkgPathSegments := strings.Split(typ.PkgPath(), "/")
		packageName := pkgPathSegments[len(pkgPathSegments)-1]
		structFullName = packageName + "." + typ.Name()
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldType := field.Type

		if fieldType.Kind() == reflect.Struct {
			for k, v := range getStructKeys(reflect.New(fieldType).Interface()) {
				result[k] = mergeAndSortUnique(result[k], v)
			}
		} else if fieldType.Kind() == reflect.Map {
			if fieldType.Key().Kind() == reflect.String {
				yamlTag := field.Tag.Get("yaml")
				if yamlTag != "" && yamlTag != "-" {
					parts := strings.Split(yamlTag, ",")
					if len(parts) > 0 {
						result[structFullName] = mergeAndSortUnique(result[structFullName], []string{parts[0]})
					}
				}
			}
			// Recurse if the value type of the map is a pointer-to-struct
			mapValueType := fieldType.Elem()
			if mapValueType.Kind() == reflect.Ptr && mapValueType.Elem().Kind() == reflect.Struct {
				for k, v := range getStructKeys(reflect.New(mapValueType.Elem()).Interface()) {
					result[k] = mergeAndSortUnique(result[k], v)
				}
			}
		} else if fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct {
			for k, v := range getStructKeys(reflect.New(fieldType.Elem()).Interface()) {
				result[k] = mergeAndSortUnique(result[k], v)
			}
		} else if fieldType.Kind() == reflect.Slice && fieldType.Elem().Kind() == reflect.Struct {
			for k, v := range getStructKeys(reflect.New(fieldType.Elem()).Interface()) {
				result[k] = mergeAndSortUnique(result[k], v)
			}
		} else {
			yamlTag := field.Tag.Get("yaml")
			if yamlTag != "" && yamlTag != "-" {
				parts := strings.Split(yamlTag, ",")
				if len(parts) > 0 {
					result[structFullName] = mergeAndSortUnique(result[structFullName], []string{parts[0]})
				}
			}
		}
	}

	return result
}
