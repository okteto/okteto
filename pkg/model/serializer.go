// Copyright 2020 The Okteto Authors
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
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/okteto/okteto/pkg/log"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

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
		e.Value = os.ExpandEnv(parts[1])
		return nil
	}

	e.Name = os.ExpandEnv(parts[0])
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (e EnvVar) MarshalYAML() (interface{}, error) {
	return e.Name + "=" + e.Value, nil
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
	// if len(c.Values) == 1 && !strings.Contains(c.Values[0], " ") {
	// 	return c.Values[0], nil
	// }
	return c.Values, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (buildInfo *BuildInfo) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		buildInfo.Name = rawString
		return nil
	}

	var rawBuildInfo BuildInfoRaw
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
		return buildInfo.BuildInfoRaw, nil
	}
	if buildInfo.Dockerfile != "" && buildInfo.Dockerfile != "./Dockerfile" {
		return buildInfo.BuildInfoRaw, nil
	}
	if buildInfo.Target != "" {
		return buildInfo.BuildInfoRaw, nil
	}
	if buildInfo.Args != nil && len(buildInfo.Args) != 0 {
		return buildInfo.BuildInfoRaw, nil
	}
	return buildInfo.Name, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (s *Secret) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	rawExpanded := os.ExpandEnv(raw)
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
		v.LocalPath = os.ExpandEnv(parts[0])
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
func (s *Sync) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.SplitN(raw, ":", 2)
	if len(parts) == 2 {
		s.LocalPath = os.ExpandEnv(parts[0])
		s.RemotePath = parts[1]
		return nil
	}

	return fmt.Errorf("each element in the 'sync' field must follow the syntax 'localPath:remotePath'")
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (s Sync) MarshalYAML() (interface{}, error) {
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
