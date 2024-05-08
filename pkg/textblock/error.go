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

package textblock

import "fmt"

type Error struct {
	Line int
}

type ErrorUnexpectedStart Error
type ErrorUnexpectedEnd Error
type ErrorMissingEnd Error

func (e *ErrorUnexpectedStart) Error() string {
	return fmt.Sprintf("error: unexpected start string at line %d", e.Line)
}

func IsErrorUnexpectedStart(err error) bool {
	_, ok := err.(*ErrorUnexpectedStart)
	return ok
}

func (e *ErrorUnexpectedEnd) Error() string {
	return fmt.Sprintf("error: unexpected end string at line %d", e.Line)
}

func IsErrorUnexpectedEnd(err error) bool {
	_, ok := err.(*ErrorUnexpectedEnd)
	return ok
}

func (e *ErrorMissingEnd) Error() string {
	return fmt.Sprintf("error: missing end string for block starting at line %d", e.Line)
}

func IsErrorMissingEnd(err error) bool {
	_, ok := err.(*ErrorMissingEnd)
	return ok
}
