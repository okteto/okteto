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
	"sort"

	"github.com/a8m/envsubst/parse"
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

type Type int

var config = map[Type]ConfigItem{
	OktetoVariableTypeBuiltIn:      {Masked: false},
	OktetoVariableTypeDotEnv:       {Masked: true},
	OktetoVariableTypeAdminAndUser: {Masked: true},
	OktetoVariableTypeFlag:         {Name: "as --var", Masked: true},
	OktetoVariableTypeLocal:        {Name: "locally or in the catalog", Masked: false},
}

type Group struct {
	Vars []Var
	Type Type
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

// LookupIncLocal returns the value of an okteto variable if it's loaded in the var manager, including local variables
func (m *Manager) LookupIncLocal(key string) (string, bool) {
	for _, g := range m.groups {
		for _, v := range g.Vars {
			if v.Name == key {
				return v.Value, true
			}
		}
	}
	return "", false
}

// LookupExcLocal returns the value of an okteto variable if it's loaded in the var manager, excluding local variables
func (m *Manager) LookupExcLocal(key string) (string, bool) {
	for _, g := range m.groups {
		if g.Type == OktetoVariableTypeLocal {
			continue
		}
		for _, v := range g.Vars {
			if v.Name == key {
				return v.Value, true
			}
		}
	}
	return "", false
}

// AddDotEnvVar allows to add a single variable to the manager of type OktetoVariableTypeDotEnv.
func (m *Manager) AddDotEnvVar(key, value string) {
	m.addVar(key, value, OktetoVariableTypeDotEnv)
}

// AddBuiltInVar allows to add a single variable to the manager of type OktetoVariableTypeBuiltIn.
func (m *Manager) AddBuiltInVar(key, value string) {
	m.addVar(key, value, OktetoVariableTypeBuiltIn)
}

// AddFlagVar allows to add a single variable to the manager of type OktetoVariableTypeFlag.
func (m *Manager) AddFlagVar(key, value string) {
	m.addVar(key, value, OktetoVariableTypeFlag)
	fmt.Println(m.groups)
}

// AddLocalVar allows to add a single variable to the manager of type OktetoVariableTypeLocal.
func (m *Manager) AddLocalVar(key, value string) {
	m.addVar(key, value, OktetoVariableTypeLocal)
}

// addVar allows to add a single variable to the manager. If other variables of the same type already exists
// the new variable will be added to the same group. If no group with the given type exists, a new group will be created.
func (m *Manager) addVar(key, value string, t Type) {
	v := Var{Name: key, Value: value}

	// if the group already exists, we append the new var to the existing group
	for i, g := range m.groups {
		if g.Type == t {
			m.groups[i].Vars = append(m.groups[i].Vars, v)
			m.m.MaskVar(value)
			// TODO: do we want this? --> m.WarnVarsPrecedence()
			return
		}
	}

	// we create a new group because the new var is the first one of its type
	m.AddGroup(Group{
		Vars: []Var{v},
		Type: t,
	})
}

// AddGroup allows to add a group of variables to the manager. The variables of the group should share the same typ.
func (m *Manager) AddGroup(g Group) {
	if config[g.Type].Masked {
		for _, v := range g.Vars {
			m.m.MaskVar(v.Value)
		}
	}

	m.groups = append(m.groups, g)
	fmt.Println(m.groups)
	m.sortGroupsByPriorityDesc()
	fmt.Println(m.groups)
}

// GetIncLocal returns an okteto variable (including local variables)
func (m *Manager) GetIncLocal(key string) string {
	val, _ := m.LookupIncLocal(key)
	return val
}

// GetExcLocal returns an okteto variable (excluding local variables)
func (m *Manager) GetExcLocal(key string) string {
	val, _ := m.LookupExcLocal(key)
	return val
}

// getGroupsExcludingType returns all groups excluding the given type
func (m *Manager) getGroupsExcludingType(typeToExclude Type) []Group {
	groups := make([]Group, 0)
	for _, g := range m.groups {
		if g.Type == typeToExclude {
			continue
		}
		groups = append(groups, g)
	}
	return groups
}

// GetOktetoVariablesExcLocal returns an array of all tifihe okteto variables that can be exported (excluding local variables)
func (m *Manager) GetOktetoVariablesExcLocal() []string {
	groups := m.getGroupsExcludingType(OktetoVariableTypeLocal)
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
	exportedVars := make(map[string]Type)

	for _, g := range m.groups {
		for _, v := range g.Vars {
			if priority, exported := exportedVars[v.Name]; exported {
				if priority > g.Type {
					prevGroupName := config[priority].Name
					currentGroupName := config[g.Type].Name
					warnings[v.Name] = fmt.Sprintf("Variable '%s' defined %s takes precedence over the same variable defined %s, which will be ignored", v.Name, currentGroupName, prevGroupName)
				}
			}
			exportedVars[v.Name] = g.Type
		}
	}

	for _, w := range warnings {
		m.m.WarningLogf(w)
	}
}

// sortGroupsByPriorityDesc sorts the groups by priority descending, so higher priority variables override lower priority ones.
func (m *Manager) sortGroupsByPriorityDesc() {
	sort.Slice(m.groups, func(i, j int) bool {
		return m.groups[i].Type > m.groups[j].Type
	})
}

// groupsToArray flattens all groups into a single array of vars. It only includes exported variables
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
	opts := &parse.Restrictions{
		NoDigit: false,
		NoEmpty: false,
		NoUnset: false,
	}
	parser := parse.New("string", envVars, opts)

	result, err := parser.Parse(s)

	return result, err
}
