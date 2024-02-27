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

package env

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Vars in groups with higher priority override those with lower priority
const (
	PriorityVarFromFlag      = 1
	PriorityVarFromLocal     = 2
	PriorityVarFromCatalog   = 3
	PriorityVarFromManifest  = 4
	PriorityVarFromDashboard = 5
)

type ConfigItem struct {
	Name   string
	Masked bool
}

var config = map[int]ConfigItem{
	PriorityVarFromFlag:      {Name: "Flag", Masked: true},
	PriorityVarFromLocal:     {Name: "Local", Masked: true},
	PriorityVarFromCatalog:   {Name: "Catalog", Masked: true},
	PriorityVarFromManifest:  {Name: "Manifest", Masked: true},
	PriorityVarFromDashboard: {Name: "Dashboard", Masked: true},
}

type LookupEnvFunc func(key string) (string, bool)
type SetEnvFunc func(key, value string) error
type MaskVarFunc func(name string)
type WarningLogFunc func(format string, args ...interface{})

type Group struct {
	Vars     []Var
	Priority int
}
type Manager struct {
	groups     []Group
	lookupEnv  LookupEnvFunc
	setEnv     SetEnvFunc
	warningLog WarningLogFunc
	maskVar    MaskVarFunc
	mu         sync.Mutex
}

// NewEnvManager creates a new environment variables manager
func NewEnvManager(lookup LookupEnvFunc, set SetEnvFunc, mask MaskVarFunc, warning WarningLogFunc) *Manager {
	return &Manager{
		lookupEnv:  lookup,
		setEnv:     set,
		maskVar:    mask,
		warningLog: warning,
	}
}

func (m *Manager) AddGroup(vars []Var, priority int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g := Group{
		Vars:     vars,
		Priority: priority,
	}
	m.groups = append(m.groups, g)

	m.mask()
	m.sort()
}

func (m *Manager) mask() {
	for _, group := range m.groups {
		isGroupMasked := config[group.Priority].Masked
		if !isGroupMasked {
			continue
		}
		for _, v := range group.Vars {
			m.maskVar(v.Value)
		}
	}
}

// sort sorts the groups by priority descending, so higher priority variables override lower priority ones.
func (m *Manager) sort() {
	sort.Slice(m.groups, func(i, j int) bool {
		return m.groups[i].Priority > m.groups[j].Priority
	})
}

// Export exports the environment variables to the current process
func (m *Manager) Export() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	exportedVars := make(map[string]int) // Track existing vars and their batch priority

	for _, g := range m.groups {
		for _, v := range g.Vars {
			if v.Name == "VAR_FROM_MANIFEST" {
				fmt.Println("debug")
			}
			minGroupsToLog := 4
			if len(m.groups) >= minGroupsToLog {
				if priority, exported := exportedVars[v.Name]; exported {
					if priority > g.Priority {
						prevGroupName := config[priority].Name
						currentGroupName := config[g.Priority].Name
						m.warningLog("Variable '%s' was already set with a lower priority (%s), overriding it with higher priority value (%s)", v.Name, prevGroupName, currentGroupName)
					}
				}
			}

			err := m.setEnv(v.Name, v.Value)
			if err != nil {
				return err
			}
			exportedVars[v.Name] = g.Priority

		}
	}
	return nil
}

func CreateGroupFromLocalVars(environ func() []string) []Var {
	envVars := environ()
	vars := make([]Var, 0, len(envVars))

	for _, envVar := range envVars {
		variableFormatParts := 2
		parts := strings.SplitN(envVar, "=", variableFormatParts)
		if len(parts) == variableFormatParts {
			vars = append(vars, Var{Name: parts[0], Value: parts[1]})
		}
	}

	return vars
}
