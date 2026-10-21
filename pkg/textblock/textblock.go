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

import (
	"strings"
)

// TextBlock defines an instance for reading and writing blocks of data
// preceded by a known header and followed by a known footer.
type TextBlock struct {
	start, end string
}

// NewTextBlock receives a footer and header strings and initializes a new
// TextBlock instance with them.
func NewTextBlock(start, end string) *TextBlock {
	return &TextBlock{
		strings.ReplaceAll(start, "\n", ""),
		strings.ReplaceAll(end, "\n", ""),
	}
}

// WriteBlock receives an input string and returns a string with the input
// wrapped by the header and footer configured in the TextBlock instance.
func (b *TextBlock) WriteBlock(input string) string {
	if input == "" {
		return strings.Join([]string{b.start, b.end}, "\n")
	}
	return strings.Join([]string{b.start, input, b.end}, "\n")
}

// FindBlocks receives an input string returns an array of strings containing
// the text of each block found
func (b *TextBlock) FindBlocks(input string) ([]string, error) {
	blocks, startFound, startLine := []string{}, false, -1
	lines := strings.Split(input, "\n")
	for i, l := range lines {
		switch l {
		case b.start:
			if startFound {
				return []string{}, &ErrorUnexpectedStart{Line: startLine}
			}
			startFound, startLine = true, i
			continue
		case b.end:
			if !startFound {
				return []string{}, &ErrorUnexpectedEnd{Line: startLine}
			}
			block := strings.Join(lines[startLine+1:i], "\n")
			blocks = append(blocks, block)
			startFound, startLine = false, -1
		}
	}

	if startFound {
		return []string{}, &ErrorMissingEnd{Line: startLine}
	}

	return blocks, nil
}
