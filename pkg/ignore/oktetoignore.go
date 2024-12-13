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

	"github.com/moby/patternmatcher"
	"github.com/moby/patternmatcher/ignorefile"
)

const RootSection = "__DEFAULT___"

type OktetoIgnorer struct {
	sections map[string]string
	mu       sync.RWMutex
}

// BuildOnly returns a new OktetoIgnorer that will only consider the build
// section AND the root section into account
func (i *OktetoIgnorer) BuildOnly() *OktetoIgnorer {
	buildSections := make(map[string]string)

	i.mu.RLock()
	defer i.mu.RUnlock()

	for k, v := range i.sections {
		if k == "build" || k == RootSection {
			buildSections[k] = v
		}
	}

	return &OktetoIgnorer{
		sections: buildSections,
	}
}

func (i *OktetoIgnorer) Get(section string) (data string) {
	i.mu.RLock()
	data = i.sections[section]
	i.mu.RUnlock()
	return
}

// Ignore matches the root section of the okteto file against the given file path
func (i *OktetoIgnorer) Ignore(filePath string) (bool, error) {
	var allSections []string
	for _, rule := range i.sections {
		allSections = append(allSections, rule)
	}
	rules, err := i.Rules(allSections...)
	if err != nil {
		return false, err
	}
	return patternmatcher.Matches(filePath, rules)
}

func (i *OktetoIgnorer) Rules(sections ...string) ([]string, error) {
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

func NewOktetoIgnorerFromFile(ignorefile string) (*OktetoIgnorer, error) {
	f, err := os.Open(ignorefile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return &OktetoIgnorer{}, nil
	}
	defer f.Close()
	return NewOktetoIgnorerFromReader(f), nil
}

func NewOktetoIgnorerFromReader(r io.Reader) *OktetoIgnorer {
	sections := sectionsFromReader(r)
	return &OktetoIgnorer{sections: sections}
}

func NewOktetoIgnoreFromFileOrNoop(oktetoIgnoreFile string) *OktetoIgnorer {
	ignore, err := NewOktetoIgnorerFromFile(oktetoIgnoreFile)
	if err != nil {
		return &OktetoIgnorer{sections: map[string]string{}}
	}
	return ignore
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
