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
	"os"

	"github.com/okteto/okteto/pkg/okteto"
)

type ContextOptions struct {
	Token        string
	Context      string
	Namespace    string
	Builder      string
	OnlyOkteto   bool
	Save         bool
	Show         bool
	isCtxCommand bool
}

func (o *ContextOptions) initFromContext() {
	if o.isCtxCommand {
		return
	}
	if o.Context != "" {
		return
	}
	ctxStore := okteto.ContextStore()
	if ctxStore.CurrentContext == "" {
		return
	}

	if okCtx, ok := ctxStore.Contexts[ctxStore.CurrentContext]; ok {
		o.Context = ctxStore.CurrentContext
		if o.Namespace == "" {
			o.Namespace = okCtx.Namespace
		}
	}
}

func (o *ContextOptions) initFromEnvVars() {
	if o.Token == "" {
		o.Token = os.Getenv("OKTETO_TOKEN")
	}

	if o.Context == "" && os.Getenv("OKTETO_URL") != "" {
		o.Context = os.Getenv("OKTETO_URL")
		o.Save = true
	}

	if o.Context == "" && os.Getenv("OKTETO_CONTEXT") != "" {
		o.Context = os.Getenv("OKTETO_CONTEXT")
		o.Save = true
	}

	if o.Token != "" {
		if o.Context == "" {
			o.Context = okteto.CloudURL
		}
		o.Save = true
	}

	if o.Namespace == "" && os.Getenv("OKTETO_NAMESPACE") != "" {
		o.Namespace = os.Getenv("OKTETO_NAMESPACE")
	}
}
