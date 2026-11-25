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

package log

import "regexp"

// githubTokenPattern matches GitHub tokens: the ones with a gh[pousr]_ prefix, including
// installation tokens that carry a JWT (dot-separated segments), and fine-grained PATs.
// Git credential helpers hand them to commands without Okteto knowing their value, so
// they are masked by their shape. A dot is only part of a token when more of it follows,
// so the period that ends a sentence stays.
var githubTokenPattern = regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_-]{30,}(?:\.[A-Za-z0-9_-]+)*|github_pat_[A-Za-z0-9_]{30,}`)
