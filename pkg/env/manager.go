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
	"sync"
)

// Vars in groups with higher priority override those with lower priority
const (
	PRIORITY_VAR_FROM_FLAG            = 1
	PRIORITY_VAR_FROM_LOCAL           = 2
	PRIORITY_VAR_FROM_CATALOG         = 3
	PRIORITY_VAR_FROM_MANIFEST        = 4
	PRIORITY_VAR_FROM_DASHBOARD_USER  = 5
	PRIORITY_VAR_FROM_DASHBOARD_ADMIN = 6
)

type ConfigItem struct {
	Name   string
	Masked bool
}

var config = map[int]ConfigItem{
	PRIORITY_VAR_FROM_FLAG:            {Name: "Flag", Masked: false},
	PRIORITY_VAR_FROM_LOCAL:           {Name: "Local", Masked: true},
	PRIORITY_VAR_FROM_CATALOG:         {Name: "Catalog", Masked: false},
	PRIORITY_VAR_FROM_MANIFEST:        {Name: "Manifest", Masked: true},
	PRIORITY_VAR_FROM_DASHBOARD_USER:  {Name: "Dashboard User", Masked: false},
	PRIORITY_VAR_FROM_DASHBOARD_ADMIN: {Name: "Dashboard Admin", Masked: true},
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
	batches    []Group
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

	batch := Group{
		Vars:     vars,
		Priority: priority,
	}
	m.batches = append(m.batches, batch)

	m.mask()
	m.sort()
}

func (m *Manager) mask() {
	for _, group := range m.batches {
		isGroupMasked := config[group.Priority].Masked
		if !isGroupMasked {
			continue
		}
		for _, v := range group.Vars {
			m.maskVar(v.Name)
		}
	}
}

// sort sorts the batches by priority descending, so higher priority variables override lower priority ones.
func (m *Manager) sort() {
	sort.Slice(m.batches, func(i, j int) bool {
		return m.batches[i].Priority > m.batches[j].Priority
	})
}

// Export exports the environment variables to the current process
func (m *Manager) Export() error {
	for _, batch := range m.batches {
		for _, v := range batch.Vars {
			err := m.setEnv(v.Name, v.Value)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
