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

package types

import (
	"github.com/okteto/okteto/pkg/model"
)

// BuildSshSession is a reference to an ssh session which translates to a
// --mount=ssh,id={id} argument in a buildkit run.
// More info here: https://github.com/moby/buildkit/blob/master/frontend/dockerfile/docs/reference.md#run---mounttypessh
type BuildSshSession struct {
	// Id is the name of the key for the mount. Defaults to "default"
	Id string
	// Target is the ssh-agent socket to mount the path to a *.pem file
	Target string
}

type HostMap struct {
	Hostname string
	IP       string
}

// BuildOptions define the options available for build
type BuildOptions struct {
	BuildArgs     []string
	CacheFrom     []string
	File          string
	NoCache       bool
	OutputMode    string
	Path          string
	Secrets       []string
	Platform      string
	Tag           string
	Target        string
	Namespace     string
	BuildToGlobal bool
	K8sContext    string
	ExportCache   []string
	// CommandArgs comes from the user input on the command
	CommandArgs  []string
	EnableStages bool

	SshSessions []BuildSshSession

	Manifest *model.Manifest
	DevTag   string

	ExtraHosts []HostMap
}
