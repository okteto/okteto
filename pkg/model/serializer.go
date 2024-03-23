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
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deps"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/externalresource"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/forward"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	maxVolumeParamDefinition   = 3
	volumeParamsWithSubPath    = 3
	volumeParamsWithoutSubpath = 2
	defaultSecretMode          = 420
)

var (

	// errDevModeNotValid is raised when development mode in manifest is not 'sync' nor 'hybrid'
	errDevModeNotValid = errors.New("development mode not valid. Value must be one of: ['sync', 'hybrid']")
)

type syncRaw struct {
	LocalPath      string
	RemotePath     string
	Folders        []SyncFolder `json:"folders,omitempty" yaml:"folders,omitempty"`
	RescanInterval int          `json:"rescanInterval,omitempty" yaml:"rescanInterval,omitempty"`
	Compression    bool         `json:"compression" yaml:"compression"`
	Verbose        bool         `json:"verbose" yaml:"verbose"`
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
	// A node selector term, associated with the corresponding weight.
	Preference NodeSelectorTerm `yaml:"preference" json:"preference"`
	// Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.
	Weight int32 `yaml:"weight" json:"weight"`
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
	TopologyKey   string         `yaml:"topologyKey" json:"topologyKey"`
	Namespaces    []string       `yaml:"namespaces,omitempty" json:"namespaces,omitempty"`
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
	PodAffinityTerm PodAffinityTerm `yaml:"podAffinityTerm" json:"podAffinityTerm"`
	Weight          int32           `yaml:"weight" json:"weight"`
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

	rawExpanded, err := env.ExpandEnv(raw)
	if err != nil {
		return err
	}
	secretWithModeLength := 3
	secretWithoutModeLength := 2
	parts := strings.Split(rawExpanded, ":")
	if runtime.GOOS == "windows" {
		if len(parts) >= secretWithModeLength {
			localPath := fmt.Sprintf("%s:%s", parts[0], parts[1])
			if filepath.IsAbs(localPath) {
				parts = append([]string{localPath}, parts[2:]...)
			}
		}
	}

	// if the secret is not formatted correctly we return an empty secret (i.e. empty LocalPath and RemotePath)
	// and we rely on the secret validation to return the appropriate error to the user
	if len(parts) < secretWithoutModeLength || len(parts) > secretWithModeLength {
		return nil
	}

	s.LocalPath = parts[0]
	s.RemotePath = parts[1]

	if len(parts) == secretWithModeLength {
		mode, err := strconv.ParseInt(parts[2], 8, 32)
		if err != nil {
			return fmt.Errorf("error parsing secret '%s' mode: %w", parts[0], err)
		}
		s.Mode = int32(mode)
	} else {
		s.Mode = defaultSecretMode
	}

	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (s Secret) MarshalYAML() (interface{}, error) {
	if s.Mode == defaultSecretMode {
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
	maxReverseParts := 2
	parts := strings.SplitN(raw, ":", maxReverseParts)
	if len(parts) != maxReverseParts {
		return fmt.Errorf("wrong port-forward syntax '%s', must be of the form 'localPort:RemotePort'", raw)
	}
	remotePort, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("cannot convert remote port '%s' in reverse '%s'", parts[0], raw)
	}

	localPort, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("cannot convert local port '%s' in reverse '%s'", parts[1], raw)
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

	maxVolumeParts := 2
	parts := strings.SplitN(raw, ":", maxVolumeParts)
	if len(parts) == maxVolumeParts {
		oktetoLog.Yellow("The syntax '%s' is deprecated in the 'volumes' field and will be removed in a future version. Use the field 'sync' instead (%s)", raw, syncFieldDocsURL)
		v.LocalPath, err = env.ExpandEnv(parts[0])
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

	windowsSyncFolderParts := 3
	syncFolderParts := 2

	parts := strings.Split(raw, ":")
	if len(parts) == syncFolderParts {
		s.LocalPath, err = env.ExpandEnv(parts[0])
		if err != nil {
			return err
		}
		s.RemotePath, err = env.ExpandEnv(parts[1])
		if err != nil {
			return err
		}
		return nil
	} else if len(parts) == windowsSyncFolderParts {
		windowsPath := fmt.Sprintf("%s:%s", parts[0], parts[1])
		s.LocalPath, err = env.ExpandEnv(windowsPath)
		if err != nil {
			return err
		}
		s.RemotePath, err = env.ExpandEnv(parts[2])
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

	parts := strings.SplitN(raw, ":", maxVolumeParamDefinition)
	switch len(parts) {
	case volumeParamsWithoutSubpath:
		v.Name = parts[0]
		v.MountPath = parts[1]
	case volumeParamsWithSubPath:
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
	Selector          Selector               `json:"selector,omitempty" yaml:"selector,omitempty"`
	UnsupportedFields map[string]interface{} `yaml:",inline" json:"-"`
	Workdir           string                 `json:"workdir,omitempty" yaml:"workdir,omitempty"`
	Mode              string                 `json:"mode,omitempty" yaml:"mode,omitempty"`
	Forward           []forward.Forward      `json:"forward,omitempty" yaml:"forward,omitempty"`
	Environment       env.Environment        `json:"environment,omitempty" yaml:"environment,omitempty"`
	Command           hybridCommand          `json:"command,omitempty" yaml:"command,omitempty"`
	Reverse           []Reverse              `json:"reverse,omitempty" yaml:"reverse,omitempty"`
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
	Name          string                   `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace     string                   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Context       string                   `json:"context,omitempty" yaml:"context,omitempty"`
	Icon          string                   `json:"icon,omitempty" yaml:"icon,omitempty"`
	Deploy        *DeployInfo              `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	Dev           ManifestDevs             `json:"dev,omitempty" yaml:"dev,omitempty"`
	Test          ManifestTests            `json:"test,omitempty" yaml:"test,omitempty"`
	Destroy       *DestroyInfo             `json:"destroy,omitempty" yaml:"destroy,omitempty"`
	Build         build.ManifestBuild      `json:"build,omitempty" yaml:"build,omitempty"`
	Dependencies  deps.ManifestSection     `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	GlobalForward []forward.GlobalForward  `json:"forward,omitempty" yaml:"forward,omitempty"`
	External      externalresource.Section `json:"external,omitempty" yaml:"external,omitempty"`

	DeprecatedDevs []string `yaml:"devs"`
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
		Build:        map[string]*build.Info{},
		Dependencies: deps.ManifestSection{},
		External:     externalresource.Section{},
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
	m.Test = manifest.Test

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

func getKeyValue(unmarshal func(interface{}) error) (map[string]string, error) {
	result := make(map[string]string)

	var rawList []env.Var
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
		value, err = env.ExpandEnv(value)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
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
