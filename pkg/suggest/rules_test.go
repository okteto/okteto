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

package suggest

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewLevenshteinRule_ErrWithoutPanic(t *testing.T) {
	inputErr := errors.New("a random test error")
	r := newLevenshteinRule("(", "test")
	err := r.apply(inputErr)

	assert.Equal(t, "a random test error", err.Error())
}
