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

package metadata

//Metadata represents what labels and annotation must be injected into dev environment
type Metadata struct {
	Labels      Labels      `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations Annotations `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}
