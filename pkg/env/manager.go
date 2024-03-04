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
	PriorityVarFromFlag     = 1
	PriorityVarFromLocal    = 2
	PriorityVarFromManifest = 3
	PriorityVarFromPlatform = 4
)

type ConfigItem struct {
	Name   string
	Masked bool
}

type Priority int

var config = map[Priority]ConfigItem{
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
	Priority Priority
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
	//exportedVars map[string]Priority
}

// NewEnvManager creates a new environment variables manager
func NewEnvManager(m ManagerInterface) *Manager {
	return &Manager{
		envManager: m,
		//exportedVars: make(map[string]Priority),
	}
}

func (m *Manager) AddGroup(vars []Var, priority Priority) {
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

	m.sortGroupsByPriorityDesc()
}

// sortGroupsByPriorityDesc sorts the groups by priority descending, so higher priority variables override lower priority ones.
func (m *Manager) sortGroupsByPriorityDesc() {
	sort.Slice(m.groups, func(i, j int) bool {
		return m.groups[i].Priority > m.groups[j].Priority
	})
}

// Export exports the environment variables to the current process
func (m *Manager) Export() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, g := range m.groups {
		for _, v := range g.Vars {
			err := m.envManager.SetEnv(v.Name, v.Value)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// WarnVarsPrecedence prints out a warning message clarifying which variables take precedence over others in case a variables has been defined in multiple groups
func (m *Manager) WarnVarsPrecedence() {
	warnings := make(map[string]string)
	exportedVars := make(map[string]Priority)

	for _, g := range m.groups {
		for _, v := range g.Vars {
			if priority, exported := exportedVars[v.Name]; exported {
				if priority > g.Priority {
					prevGroupName := config[priority].Name
					currentGroupName := config[g.Priority].Name
					warnings[v.Name] = fmt.Sprintf("Variable '%s' defined %s takes precedence over the same variable defined %s, which will be ignored", v.Name, currentGroupName, prevGroupName)
				}
			}
			exportedVars[v.Name] = g.Priority
		}
	}

	for _, w := range warnings {
		m.envManager.WarningLogf(w)
	}
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
