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

package displayer

import (
	"io"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// Displayer displays the commands from another writer to stdout
type Displayer interface {
	Display(commandName string)
	CleanUp(err error)
}

// NewDisplayer returns a new displayer
func NewDisplayer(output string, stdout, stderr io.Reader) Displayer {
	var displayer Displayer
	switch output {
	case oktetoLog.TTYFormat:
		displayer = newTTYDisplayer(stdout, stderr)
	case oktetoLog.PlainFormat:
		displayer = newPlainDisplayer(stdout, stderr)
	case oktetoLog.JSONFormat:
		displayer = newJSONDisplayer(stdout, stderr)
	default:
		displayer = newTTYDisplayer(stdout, stderr)
	}
	return displayer
}
