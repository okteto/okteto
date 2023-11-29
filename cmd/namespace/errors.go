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

package namespace

import (
	"errors"
)

var errFailedDeleteNamespace = errors.New("failed to delete namespace")

var errDeleteNamespaceTimeout = errors.New("namespace delete didn't finish")

var errFailedSleepNamespace = errors.New("failed to sleep namespace")

var errFailedWakeNamespace = errors.New("failed to wake namespace")
