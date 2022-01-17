// Copyright 2021 The Okteto Authors
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

package dev

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/constants"
	"github.com/okteto/okteto/pkg/model/environment"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type syncRaw struct {
	Compression    bool         `json:"compression" yaml:"compression"`
	Verbose        bool         `json:"verbose" yaml:"verbose"`
	RescanInterval int          `json:"rescanInterval,omitempty" yaml:"rescanInterval,omitempty"`
	Folders        []SyncFolder `json:"folders,omitempty" yaml:"folders,omitempty"`
	LocalPath      string
	RemotePath     string
}

// probesRaw represents the healthchecks info for serialization
type probesRaw struct {
	Liveness  bool `json:"liveness,omitempty" yaml:"liveness,omitempty"`
	Readiness bool `json:"readiness,omitempty" yaml:"readiness,omitempty"`
	Startup   bool `json:"startup,omitempty" yaml:"startup,omitempty"`
}

// lifecycleRaw represents the lifecycle info for serialization
type lifecycleRaw struct {
	PostStart bool `json:"postStart,omitempty" yaml:"postStart,omitempty"`
	PostStop  bool `json:"postStop,omitempty" yaml:"postStop,omitempty"`
}

type affinityRaw struct {
	NodeAffinity    *nodeAffinity    `yaml:"nodeAffinity,omitempty" json:"nodeAffinity,omitempty"`
	PodAffinity     *podAffinity     `yaml:"podAffinity,omitempty" json:"podAffinity,omitempty"`
	PodAntiAffinity *podAntiAffinity `yaml:"podAntiAffinity,omitempty" json:"podAntiAffinity,omitempty"`
}

type nodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *nodeSelector             `yaml:"requiredDuringSchedulingIgnoredDuringExecution,omitempty" json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []preferredSchedulingTerm `yaml:"preferredDuringSchedulingIgnoredDuringExecution,omitempty" json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

type nodeSelector struct {
	NodeSelectorTerms []nodeSelectorTerm `yaml:"nodeSelectorTerms" json:"nodeSelectorTerms"`
}

type nodeSelectorTerm struct {
	MatchExpressions []nodeSelectorRequirement `yaml:"matchExpressions,omitempty" json:"matchExpressions,omitempty"`
	MatchFields      []nodeSelectorRequirement `yaml:"matchFields,omitempty" json:"matchFields,omitempty"`
}

type nodeSelectorRequirement struct {
	Key      string                     `yaml:"key" json:"key"`
	Operator apiv1.NodeSelectorOperator `yaml:"operator" json:"operator"`
	Values   []string                   `yaml:"values,omitempty" json:"values,omitempty"`
}

type preferredSchedulingTerm struct {
	// Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.
	Weight int32 `yaml:"weight" json:"weight"`
	// A node selector term, associated with the corresponding weight.
	Preference nodeSelectorTerm `yaml:"preference" json:"preference"`
}

// Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).
type podAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []podAffinityTerm         `yaml:"requiredDuringSchedulingIgnoredDuringExecution,omitempty" json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `yaml:"preferredDuringSchedulingIgnoredDuringExecution,omitempty" json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).
type podAntiAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []podAffinityTerm         `yaml:"requiredDuringSchedulingIgnoredDuringExecution,omitempty" json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `yaml:"preferredDuringSchedulingIgnoredDuringExecution,omitempty" json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

type podAffinityTerm struct {
	LabelSelector *LabelSelector `yaml:"labelSelector,omitempty" json:"labelSelector,omitempty"`
	Namespaces    []string       `yaml:"namespaces,omitempty" json:"namespaces,omitempty"`
	TopologyKey   string         `yaml:"topologyKey" json:"topologyKey"`
}

type LabelSelector struct {
	MatchLabels      map[string]string          `yaml:"matchLabels,omitempty" json:"matchLabels,omitempty"`
	MatchExpressions []LabelSelectorRequirement `yaml:"matchExpressions,omitempty" json:"matchExpressions,omitempty"`
}
type LabelSelectorRequirement struct {
	Key      string                       `yaml:"key" json:"key"`
	Operator metav1.LabelSelectorOperator `yaml:"operator" json:"operator"`
	Values   []string                     `yaml:"values,omitempty" json:"values,omitempty"`
}

type WeightedPodAffinityTerm struct {
	Weight          int32           `yaml:"weight" json:"weight"`
	PodAffinityTerm podAffinityTerm `yaml:"podAffinityTerm" json:"podAffinityTerm"`
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (d *Dev) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type devType Dev //Prevent recursion
	dev := devType(*d)
	err := unmarshal(&dev)
	if err != nil {
		return fmt.Errorf("Unmarshal error: '%s'", err)
	}
	*d = Dev(dev)

	return nil
}

// MarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (d *Dev) MarshalYAML() (interface{}, error) {
	type dev Dev // prevent recursion
	toMarshall := dev(*d)
	if isDefaultProbes(d) {
		toMarshall.Probes = nil
	}
	if areAllProbesEnabled(d.Probes) {
		toMarshall.Probes = nil
		toMarshall.Healthchecks = true
	}
	if d.AreDefaultPersistentVolumeValues() {
		toMarshall.PersistentVolumeInfo = nil
	}

	return Dev(toMarshall), nil

}

func isDefaultProbes(d *Dev) bool {
	if d.Probes != nil {
		if d.Probes.Liveness || d.Probes.Readiness || d.Probes.Startup {
			return false
		}
	}
	return true
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (e *Entrypoint) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []string
	err := unmarshal(&multi)
	if err != nil {
		var single string
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		if strings.Contains(single, " && ") {
			e.Values = []string{"sh", "-c", single}
		} else {
			e.Values, err = shellquote.Split(single)
			if err != nil {
				return err
			}
		}
	} else {
		e.Values = multi
	}
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (e Entrypoint) MarshalYAML() (interface{}, error) {
	if len(e.Values) == 1 && !strings.Contains(e.Values[0], " ") {
		return e.Values[0], nil
	}
	return e.Values, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (c *Command) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []string
	err := unmarshal(&multi)
	if err != nil {
		var single string
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		if strings.Contains(single, " ") {
			c.Values = []string{"sh", "-c", single}
		} else {
			c.Values = []string{single}
		}
	} else {
		c.Values = multi
	}
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (c Command) MarshalYAML() (interface{}, error) {
	if len(c.Values) == 1 && !strings.Contains(c.Values[0], " ") {
		return c.Values[0], nil
	}
	return c.Values, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (a *Args) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []string
	err := unmarshal(&multi)
	if err != nil {
		var single string
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		a.Values = []string{single}
	} else {
		a.Values = multi
	}
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (a Args) MarshalYAML() (interface{}, error) {
	return a.Values, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (sync *Sync) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawFolders []SyncFolder
	err := unmarshal(&rawFolders)
	if err == nil {
		sync.Compression = true
		sync.Verbose = log.IsDebug()
		sync.RescanInterval = constants.DefaultSyncthingRescanInterval
		sync.Folders = rawFolders
		return nil
	}

	var rawSync syncRaw
	rawSync.Verbose = log.IsDebug()
	err = unmarshal(&rawSync)
	if err != nil {
		return err
	}

	sync.Compression = rawSync.Compression
	sync.Verbose = rawSync.Verbose
	sync.RescanInterval = rawSync.RescanInterval
	sync.Folders = rawSync.Folders
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (sync Sync) MarshalYAML() (interface{}, error) {
	if !sync.Compression && sync.RescanInterval == constants.DefaultSyncthingRescanInterval {
		return sync.Folders, nil
	}
	return syncRaw(sync), nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (f *Reverse) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("Wrong port-forward syntax '%s', must be of the form 'localPort:RemotePort'", raw)
	}
	remotePort, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("Cannot convert remote port '%s' in reverse '%s'", parts[0], raw)
	}

	localPort, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("Cannot convert local port '%s' in reverse '%s'", parts[1], raw)
	}

	f.Local = localPort
	f.Remote = remotePort
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (f Reverse) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%d:%d", f.Remote, f.Local), nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (r *ResourceList) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw map[apiv1.ResourceName]string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	if *r == nil {
		*r = ResourceList{}
	}

	for k, v := range raw {
		parsed, err := resource.ParseQuantity(v)
		if err != nil {
			return err
		}

		(*r)[k] = parsed
	}

	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (r ResourceList) MarshalYAML() (interface{}, error) {
	m := make(map[apiv1.ResourceName]string)
	for k, v := range r {
		m[k] = v.String()
	}

	return m, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (v *Volume) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.SplitN(raw, ":", 2)
	if len(parts) == 2 {
		log.Yellow("The syntax '%s' is deprecated in the 'volumes' field. Use the field 'sync' instead (%s)", raw, constants.SyncFieldDocsURL)
		v.LocalPath, err = environment.ExpandEnv(parts[0])
		if err != nil {
			return err
		}
		v.RemotePath = parts[1]
	} else {
		v.RemotePath = parts[0]
	}
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (v Volume) MarshalYAML() (interface{}, error) {
	return v.RemotePath, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (s *SyncFolder) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.Split(raw, ":")
	if len(parts) == 2 {
		s.LocalPath, err = environment.ExpandEnv(parts[0])
		if err != nil {
			return err
		}
		s.RemotePath, err = environment.ExpandEnv(parts[1])
		if err != nil {
			return err
		}
		return nil
	} else if len(parts) == 3 {
		windowsPath := fmt.Sprintf("%s:%s", parts[0], parts[1])
		s.LocalPath, err = environment.ExpandEnv(windowsPath)
		if err != nil {
			return err
		}
		s.RemotePath, err = environment.ExpandEnv(parts[2])
		if err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("each element in the 'sync' field must follow the syntax 'localPath:remotePath'")
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (s SyncFolder) MarshalYAML() (interface{}, error) {
	return s.LocalPath + ":" + s.RemotePath, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (v *ExternalVolume) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.SplitN(raw, ":", 3)
	switch len(parts) {
	case 2:
		v.Name = parts[0]
		v.MountPath = parts[1]
	case 3:
		v.Name = parts[0]
		v.SubPath = parts[1]
		v.MountPath = parts[2]
	default:
		return fmt.Errorf("external volume must follow the syntax 'name:subpath:mountpath', where subpath is optional")
	}
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (v ExternalVolume) MarshalYAML() (interface{}, error) {
	if v.SubPath == "" {
		return v.Name + ":" + v.MountPath, nil
	}
	return v.Name + ":" + v.SubPath + ":" + v.MountPath, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (p *Probes) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawBool bool
	err := unmarshal(&rawBool)
	if err == nil {
		p.Liveness = rawBool
		p.Startup = rawBool
		p.Readiness = rawBool
		return nil
	}

	var healthCheckProbesRaw probesRaw
	err = unmarshal(&healthCheckProbesRaw)
	if err != nil {
		return err
	}

	p.Liveness = healthCheckProbesRaw.Liveness
	p.Startup = healthCheckProbesRaw.Startup
	p.Readiness = healthCheckProbesRaw.Readiness
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (p Probes) MarshalYAML() (interface{}, error) {
	if p.Liveness && p.Readiness && p.Startup {
		return true, nil
	}
	return probesRaw(p), nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (l *Lifecycle) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawBool bool
	err := unmarshal(&rawBool)
	if err == nil {
		l.PostStart = rawBool
		l.PostStop = rawBool
		return nil
	}

	var lifecycleRaw lifecycleRaw
	err = unmarshal(&lifecycleRaw)
	if err != nil {
		return err
	}

	l.PostStart = lifecycleRaw.PostStart
	l.PostStop = lifecycleRaw.PostStop
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (l Lifecycle) MarshalYAML() (interface{}, error) {
	if l.PostStart && l.PostStop {
		return true, nil
	}
	return lifecycleRaw(l), nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (t *Timeout) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type timeout Timeout // prevent recursion
	var extendedNotation timeout
	err := unmarshal(&extendedNotation)
	if err != nil {
		var reducedNotation Duration
		err := unmarshal(&reducedNotation)
		if err != nil {
			return err
		}
		t.Default = time.Duration(reducedNotation)
		return nil
	}
	t.Default = extendedNotation.Default
	t.Resources = extendedNotation.Resources
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var durationString string
	err := unmarshal(&durationString)
	if err != nil {
		return err
	}
	seconds, err := strconv.Atoi(durationString)
	if err == nil {
		duration, err := time.ParseDuration(fmt.Sprintf("%ds", seconds))
		if err != nil {
			return err
		}
		*d = Duration(duration)
		return nil
	}

	var duration time.Duration
	err = unmarshal(&duration)
	if err != nil {
		return err
	}
	*d = Duration(duration)
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
// Unmarshal into our yaml affinity to marshal it into json and unmarshal it with the apiv1.Affinity.
func (a *Affinity) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var affinityRaw affinityRaw
	err := unmarshal(&affinityRaw)
	if err != nil {
		return err
	}
	bytes, err := json.Marshal(affinityRaw)
	if err != nil {
		return err
	}
	affinity := &apiv1.Affinity{}
	err = json.Unmarshal(bytes, affinity)
	if err != nil {
		return err
	}

	*a = Affinity(*affinity)

	return nil
}
