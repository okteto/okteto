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

package io

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTTYSpinner(t *testing.T) {
	sp := newTTYSpinner("test")

	sp.Start()
	sp.Stop()
	assert.Equal(t, "Test", sp.getMessage())
	assert.NotEmpty(t, sp.preUpdateFunc())
}

func TestNoSpinner(t *testing.T) {
	sp := newNoSpinner("test")

	sp.Start()
	sp.Stop()
	assert.Equal(t, "Test", sp.getMessage())
}

func TestCalculateSpinnerMessage(t *testing.T) {
	tt := []struct {
		name     string
		message  string
		width    int
		expected string
	}{
		{
			name:     "message is empty",
			message:  "",
			width:    10,
			expected: "",
		},
		{
			name:     "message is shorter than the width",
			message:  "test",
			width:    10,
			expected: "Test",
		},
		{
			name:     "width is smaller than 4",
			message:  "test",
			width:    3,
			expected: "Test",
		},
		{
			name:     "message is longer than the width",
			message:  "my spinner is special",
			width:    10,
			expected: "My sp...",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			sp := newTTYSpinner(tc.message)
			actual := sp.calculateSuffix(tc.width)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
