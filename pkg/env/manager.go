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
	"sort"
	"strings"
	"sync"
)

// Vars in groups with higher priority override those with lower priority
const (
	PriorityVarFromFlag     = 1
	PriorityVarFromLocal    = 2
	PriorityVarFromManifest = 3
	PriorityVarFromPlatform = 4
)

type ConfigItem struct {
	Name   string
	Masked bool
}

var config = map[int]ConfigItem{
	PriorityVarFromFlag:     {Name: "as --var", Masked: true},
	PriorityVarFromLocal:    {Name: "locally or in the catalog", Masked: false},
	PriorityVarFromManifest: {Name: "in the manifest", Masked: true},
	PriorityVarFromPlatform: {Name: "in the Okteto Platform", Masked: true},
}

type LookupEnvFunc func(key string) (string, bool)
type SetEnvFunc func(key, value string) error
type MaskVarFunc func(name string)
type WarningLogFunc func(format string, args ...interface{})

type group struct {
	Vars     []Var
	Priority int
}

type ManagerInterface interface {
	LookupEnv(key string) (string, bool)
	SetEnv(key, value string) error
	MaskVar(value string)
	WarningLogf(format string, args ...interface{})
}

type Manager struct {
	envManager ManagerInterface
	groups     []group
	mu         sync.Mutex
}

// NewEnvManager creates a new environment variables manager
func NewEnvManager(m ManagerInterface) *Manager {
	return &Manager{
		envManager: m,
	}
}

func (m *Manager) AddGroup(vars []Var, priority int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g := group{
		Vars:     vars,
		Priority: priority,
	}
	m.groups = append(m.groups, g)

	if config[g.Priority].Masked {
		for _, v := range vars {
			m.envManager.MaskVar(v.Value)
		}
	}

	m.sort()
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
			minGroupsToLog := 4
			if len(m.groups) >= minGroupsToLog {
				if priority, exported := exportedVars[v.Name]; exported {
					if priority > g.Priority {
						prevGroupName := config[priority].Name
						currentGroupName := config[g.Priority].Name
						m.envManager.WarningLogf("Variable '%s' defined %s takes precedence over the same variable defined %s, which will be ignored", v.Name, currentGroupName, prevGroupName)
					}
				}
			}

			err := m.envManager.SetEnv(v.Name, v.Value)
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
