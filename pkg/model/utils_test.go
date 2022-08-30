// Copyright 2022 The Okteto Authors
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
	"testing"
)

func Test_getFieldValueByTag(t *testing.T) {
	newCar := struct {
		Name              string               `json:"name" yaml:"yamlName,omitempty"`
		Flag              bool               `json:"flag" yaml:"yamlFlag,omitempty"`
	}{
		Name:    "Okteto",
		Flag: true,
	}

	var tests = []struct {
		name     string
		fieldName string
		tag string
		shouldExist bool
		expected any
	}{
		{name: "get string from struct [json tag]", fieldName: "name", tag: "json", shouldExist: true, expected: "Okteto"},
		{name: "get boolean from struct [json tag]", fieldName: "flag", tag: "json", shouldExist: true,expected: true},
		{name: "get missing key from struct [json tag]", fieldName: "missing-key", shouldExist: false, tag: "json"},
		{name: "get string from struct [yaml tag]", fieldName: "yamlName", tag: "yaml", shouldExist: true,expected: "Okteto"},
		{name: "get boolean from struct [yaml tag]", fieldName: "yamlFlag", tag: "yaml", shouldExist: true,expected: true},
		{name: "get missing key from struct [yaml tag]", fieldName: "missing-key", shouldExist: false, tag: "yaml"},
		{name: "get unknown key from unknown tag", fieldName: "unknown-key", tag: "unknown-tag", shouldExist: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, ok := GetFieldValueByTag(&newCar, tt.fieldName, tt.tag)
			if !ok && tt.shouldExist {
				t.Errorf("%s: couldn't find key: '%s'", tt.name, tt.fieldName)
			}
			
			if ok && !tt.shouldExist {
				t.Errorf("%s: found a key that doesn't exist: '%s'", tt.name, tt.fieldName)
			}

			if ok && actual != tt.expected {
				t.Errorf("'%s' got '%v' expected: '%v'", tt.name, actual, tt.expected)
			}
		})
	}
}

func Test_GetValidNameFromFolder(t *testing.T) {
	var tests = []struct {
		name     string
		folder   string
		expected string
	}{
		{name: "all lower case", folder: "lowercase", expected: "lowercase"},
		{name: "with some lower case", folder: "lowerCase", expected: "lowercase"},
		{name: "upper case", folder: "UpperCase", expected: "uppercase"},
		{name: "valid symbols", folder: "getting-started.test", expected: "getting-started-test"},
		{name: "invalid symbols", folder: "getting_$#started", expected: "getting-started"},
		{name: "current folder", folder: ".", expected: "model"},
		{name: "parent folder", folder: "..", expected: "pkg"},
		{name: "okteto folder", folder: ".okteto", expected: "model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := GetValidNameFromFolder(tt.folder)
			if err != nil {
				t.Errorf("got an error in '%s': %s", tt.name, err)
			}
			if actual != tt.expected {
				t.Errorf("'%s' got '%s' expected '%s'", tt.name, actual, tt.expected)
			}
		})
	}
}

func Test_GetValidNameFromGitRepo(t *testing.T) {
	var tests = []struct {
		name     string
		gitRepo  string
		expected string
	}{
		{name: "https url", gitRepo: "https://github.com/okteto/stacks-getting-started", expected: "stacks-getting-started"},
		{name: "https with slash at the end", gitRepo: "https://github.com/okteto/stacks-getting-started/", expected: "stacks-getting-started"},
		{name: "ssh url", gitRepo: "git@github.com:okteto/stacks-getting-started.git", expected: "stacks-getting-started"},
		{name: "ssh url with slash at the end", gitRepo: "git@github.com:okteto/stacks-getting-started.git/", expected: "stacks-getting-started"},
		{name: "https with dots", gitRepo: "https://github.com/okteto/stacks.getting.started", expected: "stacks-getting-started"},
		{name: "URL with uppers", gitRepo: "https://github.com/okteto/StacksGettingStarted", expected: "stacksgettingstarted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateURLToName(tt.gitRepo)

			if result != tt.expected {
				t.Errorf("'%s' got '%s' expected '%s'", tt.name, result, tt.expected)
			}
		})
	}

}
