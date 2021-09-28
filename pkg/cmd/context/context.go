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
	"github.com/okteto/okteto/pkg/okteto"
)

func HasBeenLogged(oktetoURL string) bool {
	cc, err := okteto.GetContexts()
	if err != nil {
		return false
	}
	_, ok := cc.Contexts[oktetoURL]
	return ok
}

func GetToken(oktetoURL string) string {
	cc, err := okteto.GetContexts()
	if err != nil {
		return ""
	}

	if octx, ok := cc.Contexts[oktetoURL]; ok {
		return octx.Token
	}
	return ""
}
