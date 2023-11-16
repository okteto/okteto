// Copyright 2023 The Okteto Authors
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
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
	"github.com/okteto/okteto/pkg/cache"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/externalresource"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/forward"
	giturls "github.com/whilp/git-urls"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (

	// errDevModeNotValid is raised when development mode in manifest is not 'sync' nor 'hybrid'
	errDevModeNotValid = errors.New("development mode not valid. Value must be one of: ['sync', 'hybrid']")
)

// BuildInfoRaw represents the build info for serialization
type buildInfoRaw struct {
	Name             string            `yaml:"name,omitempty"`
	Context          string            `yaml:"context,omitempty"`
	Dockerfile       string            `yaml:"dockerfile,omitempty"`
	CacheFrom        cache.CacheFrom   `yaml:"cache_from,omitempty"`
	Target           string            `yaml:"target,omitempty"`
	Args             BuildArgs         `yaml:"args,omitempty"`
	Image            string            `yaml:"image,omitempty"`
	VolumesToInclude []StackVolume     `yaml:"-"`
	ExportCache      cache.ExportCache `yaml:"export_cache,omitempty"`
	DependsOn        BuildDependsOn    `yaml:"depends_on,omitempty"`
	Secrets          BuildSecrets      `yaml:"secrets,omitempty"`
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

// NodeAffinity describes node affinity scheduling rules for the pod.
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
	Operator apiv1.NodeSelectorOperator `yaml:"operator" json:"operator"`
	Values   []string                   `yaml:"values,omitempty" json:"values,omitempty"`
}

type PreferredSchedulingTerm struct {
	// Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.
	Weight int32 `yaml:"weight" json:"weight"`
	// A node selector term, associated with the corresponding weight.
	Preference NodeSelectorTerm `yaml:"preference" json:"preference"`
}

// PodAffinity describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).
type PodAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `yaml:"requiredDuringSchedulingIgnoredDuringExecution,omitempty" json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `yaml:"preferredDuringSchedulingIgnoredDuringExecution,omitempty" json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PodAntiAffinity describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).
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
func (e *BuildArg) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}
	parts := strings.SplitN(raw, "=", 2)
	e.Name = parts[0]
	if len(parts) == 2 {
		e.Value = parts[1]
		return nil
	}

	e.Name, err = ExpandEnv(parts[0], true)
	if err != nil {
		return err
	}
	e.Value = parts[0]
	return nil
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
		e.Value, err = ExpandEnv(parts[1], true)
		if err != nil {
			return err
		}
		return nil
	}

	e.Name, err = ExpandEnv(parts[0], true)
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
	if err == nil {
		c.Values = multi
		return nil
	}

	var single string
	err = unmarshal(&single)
	if err != nil {
		return err
	}

	if strings.Contains(single, " ") {
		c.Values = []string{"sh", "-c", single}
	} else {
		c.Values = []string{single}
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
		sync.Verbose = oktetoLog.IsDebug()
		sync.RescanInterval = DefaultSyncthingRescanInterval
		sync.Folders = rawFolders
		return nil
	}

	var rawSync syncRaw
	rawSync.Verbose = oktetoLog.IsDebug()
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
func (d *BuildDependsOn) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		*d = BuildDependsOn{rawString}
		return nil
	}

	var rawStringList []string
	err = unmarshal(&rawStringList)
	if err == nil {
		*d = rawStringList
		return nil
	}
	return err
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
	buildInfo.Image = rawBuildInfo.Image
	buildInfo.CacheFrom = rawBuildInfo.CacheFrom
	buildInfo.ExportCache = rawBuildInfo.ExportCache
	buildInfo.DependsOn = rawBuildInfo.DependsOn
	buildInfo.Secrets = rawBuildInfo.Secrets
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (buildInfo *BuildInfo) MarshalYAML() (interface{}, error) {
	if buildInfo.Context != "" && buildInfo.Context != "." {
		return buildInfoRaw(*buildInfo), nil
	}
	if buildInfo.Dockerfile != "" && buildInfo.Dockerfile != "./Dockerfile" {
		return buildInfoRaw(*buildInfo), nil
	}
	if buildInfo.Target != "" {
		return buildInfoRaw(*buildInfo), nil
	}
	if buildInfo.Args != nil && len(buildInfo.Args) != 0 {
		return buildInfoRaw(*buildInfo), nil
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

	rawExpanded, err := ExpandEnv(raw, true)
	if err != nil {
		return err
	}
	parts := strings.Split(rawExpanded, ":")
	if runtime.GOOS == "windows" {
		if len(parts) >= 3 {
			localPath := fmt.Sprintf("%s:%s", parts[0], parts[1])
			if filepath.IsAbs(localPath) {
				parts = append([]string{localPath}, parts[2:]...)
			}
		}
	}

	// if the secret is not formatted correctly we return an empty secret (i.e. empty LocalPath and RemotePath)
	// and we rely on the secret validation to return the appropriate error to the user
	if len(parts) < 2 || len(parts) > 3 {
		return nil
	}

	s.LocalPath = parts[0]
	s.RemotePath = parts[1]

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
		oktetoLog.Yellow("The syntax '%s' is deprecated in the 'volumes' field and will be removed in a future version. Use the field 'sync' instead (%s)", raw, syncFieldDocsURL)
		v.LocalPath, err = ExpandEnv(parts[0], true)
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
		s.LocalPath, err = ExpandEnv(parts[0], true)
		if err != nil {
			return err
		}
		s.RemotePath, err = ExpandEnv(parts[1], true)
		if err != nil {
			return err
		}
		return nil
	} else if len(parts) == 3 {
		windowsPath := fmt.Sprintf("%s:%s", parts[0], parts[1])
		s.LocalPath, err = ExpandEnv(windowsPath, true)
		if err != nil {
			return err
		}
		s.RemotePath, err = ExpandEnv(parts[2], true)
		if err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("each element in the 'sync' field must follow the syntax 'localPath:remotePath'")
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (s SyncFolder) MarshalYAML() (interface{}, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return s.LocalPath + ":" + s.RemotePath, nil
	}
	relPath, err := filepath.Rel(cwd, s.LocalPath)
	if err != nil {
		return s.LocalPath + ":" + s.RemotePath, nil
	}
	return relPath + ":" + s.RemotePath, nil
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

type hybridModeInfo struct {
	Workdir     string            `json:"workdir,omitempty" yaml:"workdir,omitempty"`
	Selector    Selector          `json:"selector,omitempty" yaml:"selector,omitempty"`
	Forward     []forward.Forward `json:"forward,omitempty" yaml:"forward,omitempty"`
	Environment Environment       `json:"environment,omitempty" yaml:"environment,omitempty"`
	Command     hybridCommand     `json:"command,omitempty" yaml:"command,omitempty"`
	Reverse     []Reverse         `json:"reverse,omitempty" yaml:"reverse,omitempty"`
	Mode        string            `json:"mode,omitempty" yaml:"mode,omitempty"`

	UnsupportedFields map[string]interface{} `yaml:",inline" json:"-"`
}

type hybridCommand Command

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (hc *hybridCommand) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []string
	err := unmarshal(&multi)
	if err == nil {
		hc.Values = multi
		return nil
	}

	var single string
	err = unmarshal(&single)
	if err != nil {
		return err
	}

	cmd, err := shellquote.Split(single)
	if err != nil {
		return err
	}
	hc.Values = cmd
	return nil
}

var hybridUnsupportedFields = []string{
	"affinity",
	"context",
	"externalVolumes",
	"image",
	"imagePullPolicy",
	"initContainer",
	"initFromImage",
	"lifecycle",
	"namespace",
	"nodeSelector",
	"persistentVolume",
	"push",
	"replicas",
	"secrets",
	"securityContext",
	"serviceAccount",
	"sshServerPort",
	"sync",
	"tolerations",
	"volumes",
}

func (h *hybridModeInfo) warnHybridUnsupportedFields() string {
	var output string

	var unsupportedFieldsUsed []string
	for key := range h.UnsupportedFields {
		for _, unsupportedFieldName := range hybridUnsupportedFields {
			if key == unsupportedFieldName {
				unsupportedFieldsUsed = append(unsupportedFieldsUsed, key)
				break
			}
		}
	}

	sort.Strings(unsupportedFieldsUsed)

	if len(unsupportedFieldsUsed) > 0 {
		output = fmt.Sprintf("In hybrid mode, the field(s) '%s' specified in your manifest are ignored", strings.Join(unsupportedFieldsUsed, ", "))
	}

	return output
}

func (d *Dev) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type devType Dev // Prevent recursion
	dev := devType(*d)

	type modeInfo struct {
		Mode string `json:"mode,omitempty" yaml:"mode,omitempty"`
	}

	mode := &modeInfo{}
	err := unmarshal(mode)
	if err != nil {

		switch mode.Mode {
		case "", constants.OktetoSyncModeFieldValue:
		case constants.OktetoHybridModeFieldValue:
			{
				hybridModeDev := &hybridModeInfo{}
				err := unmarshal(hybridModeDev)
				if err != nil {
					return err
				}

				warningMsg := hybridModeDev.warnHybridUnsupportedFields()
				if warningMsg != "" {
					oktetoLog.Warning(warningMsg)
				}
			}
		default:
			{
				return errDevModeNotValid
			}
		}
	}

	err = unmarshal(&dev)
	if err != nil {
		return err
	}

	if dev.Mode == constants.OktetoHybridModeFieldValue {
		localDir, err := filepath.Abs(dev.Workdir)
		if err != nil {
			return err
		}
		info, err := os.Stat(localDir)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("dev workdir is not a dir")
		}
		dev.Workdir = localDir
		dev.Image.Name = "busybox"

	} else {
		dev.Mode = constants.OktetoSyncModeFieldValue
	}

	*d = Dev(dev)

	return nil
}

type manifestRaw struct {
	Name          string                                   `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace     string                                   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Context       string                                   `json:"context,omitempty" yaml:"context,omitempty"`
	Icon          string                                   `json:"icon,omitempty" yaml:"icon,omitempty"`
	Deploy        *DeployInfo                              `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	Dev           ManifestDevs                             `json:"dev,omitempty" yaml:"dev,omitempty"`
	Destroy       *DestroyInfo                             `json:"destroy,omitempty" yaml:"destroy,omitempty"`
	Build         ManifestBuild                            `json:"build,omitempty" yaml:"build,omitempty"`
	Dependencies  ManifestDependencies                     `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	GlobalForward []forward.GlobalForward                  `json:"forward,omitempty" yaml:"forward,omitempty"`
	External      externalresource.ExternalResourceSection `json:"external,omitempty" yaml:"external,omitempty"`

	DeprecatedDevs []string `yaml:"devs"`
}

func getRepoNameFromGitURL(repo *url.URL) string {
	repoPath := strings.Split(strings.TrimPrefix(repo.Path, "/"), "/")
	return strings.ReplaceAll(repoPath[1], ".git", "")
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (md *ManifestDependencies) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawList []string
	err := unmarshal(&rawList)
	if err == nil {
		rawMd := ManifestDependencies{}
		for _, repo := range rawList {
			r, err := giturls.Parse(repo)
			if err != nil {
				return err
			}
			name := getRepoNameFromGitURL(r)
			rawMd[name] = &Dependency{
				Repository: r.String(),
			}
		}
		*md = rawMd
		return nil
	}

	type manifestDependencies ManifestDependencies
	var rawMap manifestDependencies
	err = unmarshal(&rawMap)
	if err != nil {
		return err
	}
	*md = ManifestDependencies(rawMap)
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (d *Dependency) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		d.Repository = rawString
		return nil
	}

	type dependencyPreventRecursionType Dependency
	var dependencyRaw dependencyPreventRecursionType
	err = unmarshal(&dependencyRaw)
	if err != nil {
		return err
	}
	*d = Dependency(dependencyRaw)

	return nil
}

func (m *Manifest) UnmarshalYAML(unmarshal func(interface{}) error) error {
	dev := NewDev()
	err := unmarshal(&dev)
	if err == nil {
		*m = *NewManifestFromDev(dev)
		return nil
	}
	if !isManifestFieldNotFound(err) {
		return err
	}

	manifest := manifestRaw{
		Dev:          map[string]*Dev{},
		Build:        map[string]*BuildInfo{},
		Dependencies: map[string]*Dependency{},
		External:     externalresource.ExternalResourceSection{},
	}
	err = unmarshal(&manifest)
	if err != nil {
		return err
	}
	m.Deploy = manifest.Deploy
	m.Destroy = manifest.Destroy
	m.Dev = manifest.Dev
	m.Icon = manifest.Icon
	m.Build = manifest.Build
	m.Namespace = manifest.Namespace
	m.Context = manifest.Context
	m.IsV2 = true
	m.Dependencies = manifest.Dependencies
	m.Name = manifest.Name
	m.GlobalForward = manifest.GlobalForward
	m.External = manifest.External

	err = m.SanitizeSvcNames()
	if err != nil {
		return err
	}

	return nil
}

func (d *DeployCommand) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var command string
	err := unmarshal(&command)
	if err == nil {
		d.Command = command
		d.Name = command
		return nil
	}

	// prevent recursion
	type deployCommand DeployCommand
	var extendedCommand deployCommand
	err = unmarshal(&extendedCommand)
	if err != nil {
		return err
	}
	*d = DeployCommand(extendedCommand)
	return nil
}

func (d *DeployInfo) MarshalYAML() (interface{}, error) {
	if d.ComposeSection != nil && len(d.ComposeSection.ComposesInfo) != 0 {
		return d, nil
	}
	isCommandList := true
	for _, cmd := range d.Commands {
		if cmd.Command != cmd.Name {
			isCommandList = false
		}
	}
	if isCommandList {
		var result []string
		for _, cmd := range d.Commands {
			result = append(result, cmd.Command)
		}
		return result, nil
	}
	return d, nil
}

func (d *DeployInfo) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var commandsString []string
	err := unmarshal(&commandsString)
	if err == nil {
		d.Commands = []DeployCommand{}
		for _, cmdString := range commandsString {
			d.Commands = append(d.Commands, DeployCommand{
				Name:    cmdString,
				Command: cmdString,
			})
		}
		return nil
	}
	var commands []DeployCommand
	err = unmarshal(&commands)
	if err == nil {
		d.Commands = commands
		return nil
	}
	type deployInfoRaw DeployInfo
	var deploy deployInfoRaw
	err = unmarshal(&deploy)
	if err != nil {
		return err
	}

	*d = DeployInfo(deploy)
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (c *ComposeSectionInfo) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var composeInfoList ComposeInfoList
	err := unmarshal(&composeInfoList)
	if err == nil {
		c.ComposesInfo = composeInfoList
		return nil
	}

	type composeSectionInfoRaw ComposeSectionInfo // This is necessary to prevent recursion
	var compose composeSectionInfoRaw
	err = unmarshal(&compose)
	if err == nil {
		*c = ComposeSectionInfo(compose)
		return nil
	}
	return err
}

// MarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (c *ComposeSectionInfo) MarshalYAML() (interface{}, error) {
	if len(c.ComposesInfo) == 1 {
		return c.ComposesInfo[0], nil
	} else if len(c.ComposesInfo) > 1 {
		return c.ComposesInfo, nil
	}
	return c, nil
}

func (c *ComposeInfoList) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var singleComposeInfo ComposeInfo
	err := unmarshal(&singleComposeInfo)
	if err == nil {
		*c = []ComposeInfo{
			singleComposeInfo,
		}
		return nil
	}

	var multipleComposeInfo []ComposeInfo
	err = unmarshal(&multipleComposeInfo)
	if err == nil {
		*c = multipleComposeInfo
		return nil
	}

	return err
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (c *ComposeInfo) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		*c = ComposeInfo{File: rawString}
		return nil
	}

	type composeInfoRaw ComposeInfo
	var composeHolder composeInfoRaw
	err = unmarshal(&composeHolder)
	if err == nil {
		*c = ComposeInfo(composeHolder)
		return nil
	}
	return err
}

func (s *ServicesToDeploy) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		*s = ServicesToDeploy{
			rawString,
		}
		return nil
	}

	type servicesToDeployRaw ServicesToDeploy
	var servicesToDeployHolder servicesToDeployRaw
	err = unmarshal(&servicesToDeployHolder)
	if err == nil {
		*s = ServicesToDeploy(servicesToDeployHolder)
		return nil
	}
	return err
}

type devRaw Dev

func (d *devRaw) UnmarshalYAML(unmarshal func(interface{}) error) error {
	devRawPointer := NewDev()
	err := unmarshal(devRawPointer)
	if err != nil {
		return err
	}

	*d = devRaw(*devRawPointer)
	return nil
}

func (d *ManifestDevs) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type manifestDevsList []string
	devsList := manifestDevsList{}
	err := unmarshal(&devsList)
	if err == nil {
		return nil
	}
	type manifestDevs map[string]devRaw
	devs := make(manifestDevs)
	err = unmarshal(&devs)
	if err != nil {
		return err
	}
	result := ManifestDevs{}

	for i := range devs {
		dev := Dev(devs[i])
		devPointer := &dev
		result[i] = devPointer
	}
	*d = result
	return nil
}

func isManifestFieldNotFound(err error) bool {
	manifestFields := []string{"devs", "dev", "name", "icon", "variables", "deploy", "destroy", "build", "namespace", "context", "dependencies"}
	for _, field := range manifestFields {
		if strings.Contains(err.Error(), fmt.Sprintf("field %s not found", field)) {
			return true
		}
	}
	return false
}

func (d *DestroyInfo) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var commandsString []string
	err := unmarshal(&commandsString)
	if err == nil {
		d.Commands = []DeployCommand{}
		for _, cmdString := range commandsString {
			d.Commands = append(d.Commands, DeployCommand{
				Name:    cmdString,
				Command: cmdString,
			})
		}
		return nil
	}
	var commands []DeployCommand
	err = unmarshal(&commands)
	if err == nil {
		d.Commands = commands
		return nil
	}
	type destroyInfoRaw DestroyInfo
	var destroy destroyInfoRaw
	err = unmarshal(&destroy)
	if err != nil {
		return err
	}

	*d = DestroyInfo(destroy)
	return nil
}

func (d *DestroyInfo) MarshalYAML() (interface{}, error) {
	isCommandList := true
	for _, cmd := range d.Commands {
		if cmd.Command != cmd.Name {
			isCommandList = false
		}
	}
	if isCommandList {
		var result []string
		for _, cmd := range d.Commands {
			result = append(result, cmd.Command)
		}
		return result, nil
	}
	return d, nil
}

func (m *Manifest) MarshalYAML() (interface{}, error) {
	if m.Destroy == nil || len(m.Destroy.Commands) == 0 {
		m.Destroy = nil
		return m, nil
	}
	return m, nil
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
	if toMarshall.ImagePullPolicy == apiv1.PullAlways {
		toMarshall.ImagePullPolicy = ""
	}
	if toMarshall.Lifecycle != nil && (!toMarshall.Lifecycle.PostStart || !toMarshall.Lifecycle.PostStop) {
		toMarshall.Lifecycle = nil
	}
	if toMarshall.Metadata != nil && len(toMarshall.Metadata.Annotations) == 0 && len(toMarshall.Metadata.Labels) == 0 {
		toMarshall.Metadata = nil
	}
	if toMarshall.InitContainer.Image == OktetoBinImageTag {
		toMarshall.InitContainer.Image = ""
	}
	if toMarshall.Timeout.Default == 1*time.Minute && toMarshall.Timeout.Resources == 2*time.Minute {
		toMarshall.Timeout.Default = 0
		toMarshall.Timeout.Resources = 0
	}
	if toMarshall.Sync.RescanInterval == DefaultSyncthingRescanInterval && toMarshall.Sync.Compression {
		toMarshall.Sync.Compression = false
	}
	if toMarshall.Interface == Localhost || toMarshall.Interface == PrivilegedLocalhost {
		toMarshall.Interface = ""
	}
	if toMarshall.SSHServerPort == oktetoDefaultSSHServerPort {
		toMarshall.SSHServerPort = 0
	}
	if toMarshall.Workdir == "/okteto" {
		toMarshall.Workdir = ""
	}
	if isDefaultSecurityContext((*Dev)(&toMarshall)) {
		toMarshall.SecurityContext = nil
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
func isDefaultSecurityContext(d *Dev) bool {
	if d.SecurityContext == nil {
		return true
	}
	if d.SecurityContext.Capabilities != nil {
		return false
	}
	if d.SecurityContext.RunAsNonRoot != nil {
		return false
	}
	if d.SecurityContext.FSGroup != nil && *d.SecurityContext.FSGroup != 0 {
		return false
	}
	if d.SecurityContext.RunAsGroup != nil && *d.SecurityContext.RunAsGroup != 0 {
		return false
	}
	if d.SecurityContext.RunAsUser != nil && *d.SecurityContext.RunAsUser != 0 {
		return false
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

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (ba *BuildArgs) UnmarshalYAML(unmarshal func(interface{}) error) error {
	buildArgs := make(BuildArgs, 0)
	result, err := getBuildArgs(unmarshal)
	if err != nil {
		return err
	}
	for key, value := range result {
		buildArgs = append(buildArgs, BuildArg{Name: key, Value: value})
	}
	sort.SliceStable(buildArgs, func(i, j int) bool {
		return strings.Compare(buildArgs[i].Name, buildArgs[j].Name) < 0
	})
	*ba = buildArgs
	return nil
}

func getBuildArgs(unmarshal func(interface{}) error) (map[string]string, error) {
	result := make(map[string]string)

	var rawList []BuildArg
	err := unmarshal(&rawList)
	if err == nil {
		for _, buildArg := range rawList {
			value, err := ExpandEnv(buildArg.Value, false)
			if err != nil {
				return nil, err
			}
			result[buildArg.Name] = value
		}
		return result, nil
	}
	var rawMap map[string]string
	err = unmarshal(&rawMap)
	if err != nil {
		return nil, err
	}
	for key, value := range rawMap {
		result[key], err = ExpandEnv(value, false)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
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
		value, err = ExpandEnv(value, true)
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
