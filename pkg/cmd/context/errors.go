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

package context

import (
	"fmt"
)

var (
	//ErrTokenFlagNeeded is raised when the command is executed from inside a pod
	ErrTokenFlagNeeded = fmt.Errorf("this command is not supported without the '--token' flag from inside a pod")

	//ErrInvalidCluster is raised when the cluster selected is not defined on your kubeconfig or is not a okteto cluster
	ErrInvalidCluster = "'%s' is a invalid cluster."

	//ErrNamespaceNotFound is raised when the namespace is not found on an okteto instance
	ErrNamespaceNotFound = "namespace '%s' not found. Please verify that the namespace exists and that you have access to it"
)
