// Copyright 2026 The Okteto Authors
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

package remote

import (
	"fmt"
	"strings"

	"al.essio.dev/pkg/shellescape"
)

// CommandFlag returns a flag for Params.CommandFlags. The flags go into a shell-form RUN instruction of the
// Dockerfile, so the value is quoted and remote-run gets it with nothing expanded
func CommandFlag(name, value string) string {
	return fmt.Sprintf("--%s %s", name, quoteForDockerfile(value))
}

// quoteForDockerfile returns the value as one word in single quotes, which nothing expands: neither /bin/sh in a RUN
// instruction nor BuildKit in an ENV instruction. A newline would end the instruction, so it becomes the two
// characters \n
func quoteForDockerfile(value string) string {
	return shellescape.Quote(strings.ReplaceAll(value, "\n", `\n`))
}
