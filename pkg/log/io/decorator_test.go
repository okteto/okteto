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

func TestTTYDecorator(t *testing.T) {
	decorator := newTTYDecorator()
	msg := "text message"

	output := decorator.Information(msg)
	assert.Equal(t, " i  text message\n", output)

	output = decorator.Warning(msg)
	assert.Equal(t, " !  text message\n", output)

	output = decorator.Success(msg)
	assert.Equal(t, " ✓  text message\n", output)

	output = decorator.Question(msg)
	assert.Equal(t, " ?  text message", output)
}

func TestPlainDecorator(t *testing.T) {
	decorator := newPlainDecorator()
	msg := "text message"

	output := decorator.Information(msg)
	assert.Equal(t, "INFO: text message\n", output)

	output = decorator.Warning(msg)
	assert.Equal(t, "WARNING: text message\n", output)

	output = decorator.Success(msg)
	assert.Equal(t, "SUCCESS: text message\n", output)

	output = decorator.Question(msg)
	assert.Equal(t, "text message", output)
}
