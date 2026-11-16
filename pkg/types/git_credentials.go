// Copyright 2026 The Okteto Authors
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

import "time"

// GitCredentials are the credentials Okteto gives for a Git repository.
// Without credentials, Password is empty and Reason says why
type GitCredentials struct {
	// ExpiresAt is zero when the expiry is unknown
	ExpiresAt time.Time
	Username  string
	Password  string
	// Reason is a sentence for the user, set when there are no credentials
	Reason string
}
