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

package vars

import (
	"fmt"
	"github.com/a8m/envsubst/parse"
	"sort"
)

// GlobalVarManager is the global instance of the Okteto Variables manager. It should only be used in the serializer where it's harder to inject the manager.
var GlobalVarManager *Manager

// Vars in groups with higher priority override those with lower priority
const (
	OktetoVariableTypeBuiltIn      = 1
	OktetoVariableTypeFlag         = 2
	OktetoVariableTypeLocal        = 3
	OktetoVariableTypeDotEnv       = 4
	OktetoVariableTypeAdminAndUser = 5
)

type ConfigItem struct {
	Name   string
	Masked bool
}

type Priority int

var config = map[Priority]ConfigItem{
	OktetoVariableTypeBuiltIn:      {Masked: false},
	OktetoVariableTypeDotEnv:       {Masked: true},
	OktetoVariableTypeAdminAndUser: {Masked: true},
	OktetoVariableTypeFlag:         {Name: "as --var", Masked: true},
	OktetoVariableTypeLocal:        {Name: "locally or in the catalog", Masked: false},
}

type Group struct {
	Vars     []Var
	Priority Priority
}

type ManagerInterface interface {
	MaskVar(value string)
	WarningLogf(format string, args ...interface{})
}

type Manager struct {
	m      ManagerInterface
	groups []Group
}

// NewVarsManager creates a new Okteto Variables manager
func NewVarsManager(m ManagerInterface) *Manager {
	return &Manager{
		m: m,
	}
}

// Lookup returns the value of the environment variable named by the key
func (m *Manager) Lookup(key string) (string, bool) {
	for _, g := range m.groups {
		for _, v := range g.Vars {
			if v.Name == key {
				return v.Value, true
			}
		}
	}
	return "", false
}

func (m *Manager) AddGroup(g Group) error {
	if config[g.Priority].Masked {
		for _, v := range g.Vars {
			m.m.MaskVar(v.Value)
		}
	}

	m.groups = append(m.groups, g)
	m.sortGroupsByPriorityDesc()

	return nil
}

// GetOktetoVariablesExcLocal returns an array of all tifihe okteto variables that can be exported (excluding local variables)
func (m *Manager) GetOktetoVariablesExcLocal() []string {
	groups := make([]Group, 0)
	for _, g := range m.groups {
		if g.Priority == OktetoVariableTypeLocal {
			continue
		}
		groups = append(groups, g)
	}
	return m.groupsToArray(groups)
}

// ExpandIncLocal replaces the variables in the given string with their values and returns the result. It expands with all groups, including local variables.
func (m *Manager) ExpandIncLocal(s string) (string, error) {
	return m.expandString(s, m.groupsToArray(m.groups))
}

// ExpandExcLocal replaces the variables in the given string with their values and returns the result. It expands with all groups, excluding local variables.
func (m *Manager) ExpandExcLocal(s string) (string, error) {
	return m.expandString(s, m.GetOktetoVariablesExcLocal())
}

// ExpandExcLocalIfNotEmpty replaces the variables in the given string with their values and returns the result. It expands with all groups, excluding local variables. If the result is an empty string, it returns the original value.
func (m *Manager) ExpandExcLocalIfNotEmpty(s string) (string, error) {
	result, err := m.expandString(s, m.GetOktetoVariablesExcLocal())
	if err != nil {
		return "", err
	}
	if result == "" {
		return s, nil
	}
	return result, nil
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
		m.m.WarningLogf(w)
	}
}

// sortGroupsByPriorityDesc sorts the groups by priority descending, so higher priority variables override lower priority ones.
func (m *Manager) sortGroupsByPriorityDesc() {
	sort.Slice(m.groups, func(i, j int) bool {
		return m.groups[i].Priority > m.groups[j].Priority
	})
}

// groupsToArray flattens all groups into a single array of vars. By defaul it only includes exported variables
func (m *Manager) groupsToArray(groups []Group) []string {
	vars := make([]string, 0)
	for _, g := range groups {
		for _, v := range g.Vars {
			vars = append(vars, v.String())
		}
	}
	return vars
}

func (m *Manager) expandString(s string, envVars []string) (string, error) {
	return parse.New("string", envVars, &parse.Restrictions{NoDigit: false, NoEmpty: false, NoUnset: false}).Parse(s)
}
