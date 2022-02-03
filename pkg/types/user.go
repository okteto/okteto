// Copyright 2022 The Okteto Authors
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

// User contains the auth information of the logged in user
type User struct {
	Name            string
	Namespace       string
	Email           string
	ExternalID      string
	Token           string
	ID              string
	New             bool
	Buildkit        string
	Registry        string
	Certificate     string
	GlobalNamespace string
	Analytics       bool
}
