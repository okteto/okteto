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
	"sort"

	"github.com/a8m/envsubst/parse"
)

// GlobalVarManager is the global instance of the Okteto Variables manager. It should only be used in the serializer where it's harder to inject the manager.
var GlobalVarManager *Manager

// Okteto CLI supports the following variable types. Each type has a different priority. Higher priority variables override lower priority ones.
const (
	OktetoVariableTypeBuiltIn      = 1
	OktetoVariableTypeFlag         = 2
	OktetoVariableTypeLocal        = 3
	OktetoVariableTypeDotEnv       = 4
	OktetoVariableTypeAdminAndUser = 5

	// OktetoSupportLocalVariablesEnabled is a feature flag that can be used to enable the support for local environment
	// variables. Once set to true, the varManager will always return local vars in Get or Lookup methods.
	OktetoSupportLocalVariablesEnabled = "OKTETO_SUPPORT_LOCAL_VARIABLES_ENABLED"
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
	OktetoVariableTypeFlag:         {Masked: true},
	OktetoVariableTypeLocal:        {Masked: false},
}

type Group struct {
	Vars []Var
	Type Type
}

// ManagerInterface is the interface that the Okteto Variables manager should implement
type ManagerInterface interface {
	MaskVar(value string)
	IsLocalVarSupportEnabled() bool
	IsLocalVarException(key string) bool
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

// LookupExcLocal returns the value of an okteto variable if it's loaded in the var manager, by default excluding
// local variables, unless the feature flag is enabled or the variable is an exception
func (m *Manager) LookupExcLocal(key string) (string, bool) {
	for _, g := range m.groups {
		if g.Type == OktetoVariableTypeLocal && !m.shouldIncludeLocalVar(key) {
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

// shouldIncludeLocalVar returns whether a given variable should be returned. This function should only be used with
// local variables. If the feature flag is enabled, all local variables should be returned. If the feature flag is disabled,
// only the exceptions should be returned.
func (m *Manager) shouldIncludeLocalVar(key string) bool {
	if m.m.IsLocalVarSupportEnabled() {
		return true
	}

	if m.m.IsLocalVarException(key) {
		return true
	}

	return false
}

// AddAdminAndUserVar allows to add a single variable to the manager of type OktetoVariableTypeAdminAndUser.
func (m *Manager) AddAdminAndUserVar(key, value string) {
	m.addVar(key, value, OktetoVariableTypeAdminAndUser)
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
}

// AddLocalVar allows to add a single variable to the manager of type OktetoVariableTypeLocal.
func (m *Manager) AddLocalVar(key, value string) {
	m.addVar(key, value, OktetoVariableTypeLocal)
}

// addVar allows to add a single variable to the manager. If other variables of the same type already exists
// the new variable will be added to the same group. If no group with the given type exists, a new group will be created.
func (m *Manager) addVar(key, value string, t Type) {
	v := Var{Name: key, Value: value}

	// we check if the group already exists
	for i, g := range m.groups {
		if g.Type == t {
			// if the var already exists, we override its value
			for j, v := range g.Vars {
				if v.Name == key {
					m.groups[i].Vars[j].Value = value
					m.maskVar(value, t)
					return
				}
			}
			// if the var does not exist, we add it to the group
			m.groups[i].Vars = append(m.groups[i].Vars, v)
			m.maskVar(value, t)
			return
		}
	}

	// we create a new group because the new var is the first one of its type
	m.AddGroup(Group{
		Vars: []Var{v},
		Type: t,
	})
}

func (m *Manager) maskVar(value string, t Type) {
	if value == "" || !config[t].Masked {
		return
	}
	m.m.MaskVar(value)
}

// AddGroup allows to add a group of variables to the manager. The variables of the group should share the same typ.
func (m *Manager) AddGroup(g Group) {
	for _, v := range g.Vars {
		m.maskVar(v.Value, g.Type)
	}

	m.groups = append(m.groups, g)
	m.sortGroupsByPriorityAsc()
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

// GetOktetoVariablesExcLocal returns an array of all the okteto variables that can be exported (excluding local variables)
func (m *Manager) GetOktetoVariablesExcLocal() []string {
	varsMap := make(map[string]struct{})
	vars := make([]string, 0)
	for _, g := range m.groups {
		for _, v := range g.Vars {
			if g.Type == OktetoVariableTypeLocal && !m.shouldIncludeLocalVar(v.Name) {
				continue
			}
			if _, exists := varsMap[v.Name]; !exists {
				vars = append(vars, v.String())
				varsMap[v.Name] = struct{}{}
			}
		}
	}
	return vars
}

// ExpandIncLocal replaces the variables in the given string with their values and returns the result. It expands with all groups, including local variables.
func (m *Manager) ExpandIncLocal(s string) (string, error) {
	varsMap := make(map[string]struct{})
	vars := make([]string, 0)
	for _, g := range m.groups {
		for _, v := range g.Vars {
			if _, exists := varsMap[v.Name]; !exists {
				vars = append(vars, v.String())
				varsMap[v.Name] = struct{}{}
			}
		}
	}

	return m.expandString(s, vars)
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

// sortGroupsByPriorityAsc sorts the groups by priority ascending, so higher priority variables override lower priority ones.
func (m *Manager) sortGroupsByPriorityAsc() {
	sort.Slice(m.groups, func(i, j int) bool {
		return m.groups[i].Type < m.groups[j].Type
	})
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
