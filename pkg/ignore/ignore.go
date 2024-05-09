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

package ignore

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/moby/patternmatcher/ignorefile"
)

const RootSection = "__DEFAULT___"

type Ignore struct {
	sections map[string]string
	mu       sync.RWMutex
}

func (i *Ignore) Get(section string) (data string) {
	i.mu.RLock()
	data = i.sections[section]
	i.mu.RUnlock()
	return
}

func (i *Ignore) Rules(sections ...string) ([]string, error) {
	var rules []string
	for _, section := range sections {
		data := i.Get(section)
		r := strings.NewReader(data)
		slice, err := ignorefile.ReadAll(r)
		if err != nil {
			return nil, err
		}
		rules = append(rules, slice...)
	}
	return rules, nil
}

func NewFromFile(ignorefile string) (*Ignore, error) {
	f, err := os.Open(ignorefile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return &Ignore{}, nil
	}
	defer f.Close()
	return NewFromReader(f), nil
}

func NewFromReader(r io.Reader) *Ignore {
	sections := sectionsFromReader(r)
	return &Ignore{sections: sections}
}

func sectionsFromReader(r io.Reader) map[string]string {
	sections := make(map[string]string)

	current := RootSection
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		ok, key := isSectionHeader(line)
		if ok {
			current = key
		} else {
			sections[current] += fmt.Sprintln(line)
		}
	}
	return sections
}

var sectionHeaderRegex = regexp.MustCompile(`^\[(?P<key>.*)\]$`)

const matchThreshold = 2

func isSectionHeader(line string) (bool, string) {
	matches := sectionHeaderRegex.FindStringSubmatch(line)
	if len(matches) < matchThreshold {
		return false, ""
	}
	return true, matches[1]
}
