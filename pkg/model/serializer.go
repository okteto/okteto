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

package model

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
	"github.com/okteto/okteto/pkg/log"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildInfoRaw represents the build info for serialization
type buildInfoRaw struct {
	Name       string      `yaml:"name,omitempty"`
	Context    string      `yaml:"context,omitempty"`
	Dockerfile string      `yaml:"dockerfile,omitempty"`
	CacheFrom  []string    `yaml:"cache_from,omitempty"`
	Target     string      `yaml:"target,omitempty"`
	Args       Environment `yaml:"args,omitempty"`
}

type syncRaw struct {
	Compression    bool         `json:"compression" yaml:"compression"`
	Verbose        bool         `json:"verbose" yaml:"verbose"`
	RescanInterval int          `json:"rescanInterval,omitempty" yaml:"rescanInterval,omitempty"`
	Folders        []SyncFolder `json:"folders,omitempty" yaml:"folders,omitempty"`
	LocalPath      string
	RemotePath     string
}

type storageResourceRaw struct {
	Size  Quantity `json:"size,omitempty" yaml:"size,omitempty"`
	Class string   `json:"class,omitempty" yaml:"class,omitempty"`
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

type AffinityRaw struct {
	NodeAffinity    *NodeAffinity    `yaml:"nodeAffinity,omitempty" json:"nodeAffinity,omitempty"`
	PodAffinity     *PodAffinity     `yaml:"podAffinity,omitempty" json:"podAffinity,omitempty"`
	PodAntiAffinity *PodAntiAffinity `yaml:"podAntiAffinity,omitempty" json:"podAntiAffinity,omitempty"`
}

// Describes node affinity scheduling rules for the pod.
type NodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelector             `yaml:"requiredDuringSchedulingIgnoredDuringExecution,omitempty" json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `yaml:"preferredDuringSchedulingIgnoredDuringExecution,omitempty" json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

type NodeSelector struct {
	NodeSelectorTerms []NodeSelectorTerm `yaml:"nodeSelectorTerms" json:"nodeSelectorTerms"`
}

type NodeSelectorTerm struct {
	MatchExpressions []NodeSelectorRequirement `yaml:"matchExpressions,omitempty" json:"matchExpressions,omitempty"`
	MatchFields      []NodeSelectorRequirement `yaml:"matchFields,omitempty" json:"matchFields,omitempty"`
}

type NodeSelectorRequirement struct {
	Key      string                     `yaml:"key" json:"key"`
	Operator apiv1.NodeSelectorOperator `yaml:"operator" json:"operator`
	Values   []string                   `yaml:"values,omitempty" json:"values,omitempty"`
}

type PreferredSchedulingTerm struct {
	// Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.
	Weight int32 `yaml:"weight" json:"weight"`
	// A node selector term, associated with the corresponding weight.
	Preference NodeSelectorTerm `yaml:"preference" json:"preference"`
}

// Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).
type PodAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `yaml:"requiredDuringSchedulingIgnoredDuringExecution,omitempty" json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `yaml:"preferredDuringSchedulingIgnoredDuringExecution,omitempty" json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).
type PodAntiAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `yaml:"requiredDuringSchedulingIgnoredDuringExecution,omitempty" json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `yaml:"preferredDuringSchedulingIgnoredDuringExecution,omitempty" json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

type PodAffinityTerm struct {
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
	PodAffinityTerm PodAffinityTerm `yaml:"podAffinityTerm" json:"podAffinityTerm"`
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (e *EnvVar) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.SplitN(raw, "=", 2)
	e.Name = parts[0]
	if len(parts) == 2 {
		e.Value, err = ExpandEnv(parts[1])
		if err != nil {
			return err
		}
		return nil
	}

	e.Name, err = ExpandEnv(parts[0])
	if err != nil {
		return err
	}
	e.Value = os.Getenv(e.Name)
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (e EnvVar) MarshalYAML() (interface{}, error) {
	return e.Name + "=" + e.Value, nil
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
		sync.Verbose = log.IsDebug()
		sync.RescanInterval = DefaultSyncthingRescanInterval
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
	if !sync.Compression && sync.RescanInterval == DefaultSyncthingRescanInterval {
		return sync.Folders, nil
	}
	return syncRaw(sync), nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (buildInfo *BuildInfo) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		buildInfo.Name = rawString
		return nil
	}

	var rawBuildInfo buildInfoRaw
	err = unmarshal(&rawBuildInfo)
	if err != nil {
		return err
	}

	buildInfo.Name = rawBuildInfo.Name
	buildInfo.Context = rawBuildInfo.Context
	buildInfo.Dockerfile = rawBuildInfo.Dockerfile
	buildInfo.Target = rawBuildInfo.Target
	buildInfo.Args = rawBuildInfo.Args
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (buildInfo BuildInfo) MarshalYAML() (interface{}, error) {
	if buildInfo.Context != "" && buildInfo.Context != "." {
		return buildInfoRaw(buildInfo), nil
	}
	if buildInfo.Dockerfile != "" && buildInfo.Dockerfile != "./Dockerfile" {
		return buildInfoRaw(buildInfo), nil
	}
	if buildInfo.Target != "" {
		return buildInfoRaw(buildInfo), nil
	}
	if buildInfo.Args != nil && len(buildInfo.Args) != 0 {
		return buildInfoRaw(buildInfo), nil
	}
	return buildInfo.Name, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (s *StorageResource) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawQuantity Quantity
	err := unmarshal(&rawQuantity)
	if err == nil {
		s.Size = rawQuantity
		return nil
	}

	var rawStorageResource storageResourceRaw
	err = unmarshal(&rawStorageResource)
	if err != nil {
		return err
	}

	s.Size = rawStorageResource.Size
	s.Class = rawStorageResource.Class
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (q *Quantity) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		qK8s, err := resource.ParseQuantity(rawString)
		if err != nil {
			return err
		}
		q.Value = qK8s
		return nil
	}

	var rawQuantity resource.Quantity
	err = unmarshal(&rawQuantity)
	if err != nil {
		return err
	}

	q.Value = rawQuantity
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (q Quantity) MarshalYAML() (interface{}, error) {
	return q.Value.String(), nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (s *Secret) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	rawExpanded, err := ExpandEnv(raw)
	if err != nil {
		return err
	}
	parts := strings.Split(rawExpanded, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("secrets must follow the syntax 'LOCAL_PATH:REMOTE_PATH:MODE'")
	}
	s.LocalPath = parts[0]
	if err := checkFileAndNotDirectory(s.LocalPath); err != nil {
		return err
	}
	s.RemotePath = parts[1]
	if !strings.HasPrefix(s.RemotePath, "/") {
		return fmt.Errorf("Secret remote path '%s' must be an absolute path", s.RemotePath)
	}
	if len(parts) == 3 {
		mode, err := strconv.ParseInt(parts[2], 8, 32)
		if err != nil {
			return fmt.Errorf("error parsing secret '%s' mode: %s", parts[0], err)
		}
		s.Mode = int32(mode)
	} else {
		s.Mode = 420
	}
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (s Secret) MarshalYAML() (interface{}, error) {
	if s.Mode == 420 {
		return fmt.Sprintf("%s:%s:%s", s.LocalPath, s.RemotePath, strconv.FormatInt(int64(s.Mode), 8)), nil
	}
	return fmt.Sprintf("%s:%s", s.LocalPath, s.RemotePath), nil
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
		log.Yellow("The syntax '%s' is deprecated in the 'volumes' field. Use the field 'sync' instead (%s)", raw, syncFieldDocsURL)
		v.LocalPath, err = ExpandEnv(parts[0])
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
		s.LocalPath, err = ExpandEnv(parts[0])
		if err != nil {
			return err
		}
		s.RemotePath = parts[1]
		return nil
	} else if len(parts) == 3 {
		windowsPath := fmt.Sprintf("%s:%s", parts[0], parts[1])
		s.LocalPath, err = ExpandEnv(windowsPath)
		if err != nil {
			return err
		}
		s.RemotePath = parts[2]
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

func checkFileAndNotDirectory(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fileInfo.Mode().IsRegular() {
		return nil
	}
	return fmt.Errorf("Secret '%s' is not a regular file", path)
}

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
func (endpoint *Endpoint) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rules []EndpointRule
	err := unmarshal(&rules)
	endpoint.Labels = make(Labels)
	if err == nil {
		endpoint.Rules = rules
		endpoint.Annotations = make(map[string]string)
		return nil
	}
	type endpointType Endpoint // prevent recursion
	var endpointRaw endpointType
	err = unmarshal(&endpointRaw)
	if err != nil {
		return err
	}

	endpoint.Rules = endpointRaw.Rules
	endpoint.Annotations = endpointRaw.Annotations
	if endpoint.Annotations == nil {
		endpoint.Annotations = make(Annotations)
	}
	for key, value := range endpointRaw.Labels {
		endpoint.Annotations[key] = value
	}

	return nil
}

func (l *Labels) UnmarshalYAML(unmarshal func(interface{}) error) error {
	labels := make(Labels)
	result, err := getKeyValue(unmarshal)
	if err != nil {
		return err
	}
	for key, value := range result {
		labels[key] = value
	}
	*l = labels
	return nil
}

func (a *Annotations) UnmarshalYAML(unmarshal func(interface{}) error) error {
	annotations := make(Annotations)
	result, err := getKeyValue(unmarshal)
	if err != nil {
		return err
	}
	for key, value := range result {
		annotations[key] = value
	}
	*a = annotations
	return nil
}

func (e *Environment) UnmarshalYAML(unmarshal func(interface{}) error) error {
	envs := make(Environment, 0)
	result, err := getKeyValue(unmarshal)
	if err != nil {
		return err
	}
	for key, value := range result {
		envs = append(envs, EnvVar{Name: key, Value: value})
	}
	sort.SliceStable(envs, func(i, j int) bool {
		return strings.Compare(envs[i].Name, envs[j].Name) < 0
	})
	*e = envs
	return nil
}

func getKeyValue(unmarshal func(interface{}) error) (map[string]string, error) {
	result := make(map[string]string)

	var rawList []EnvVar
	err := unmarshal(&rawList)
	if err == nil {
		for _, label := range rawList {
			result[label.Name] = label.Value
		}
		return result, nil
	}
	var rawMap map[string]string
	err = unmarshal(&rawMap)
	if err != nil {
		return nil, err
	}
	for key, value := range rawMap {
		value, err = ExpandEnv(value)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (envFiles *EnvFiles) UnmarshalYAML(unmarshal func(interface{}) error) error {
	result := make(EnvFiles, 0)
	var single string
	err := unmarshal(&single)
	if err != nil {
		var multi []string
		err := unmarshal(&multi)
		if err != nil {
			return err
		}
		result = multi
		*envFiles = result
		return nil
	}

	result = append(result, single)
	*envFiles = result
	return nil
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
	var affinityRaw AffinityRaw
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
