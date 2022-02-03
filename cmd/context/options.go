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

package context

import (
	"os"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

type ContextOptions struct {
	Token            string
	Context          string
	Namespace        string
	Builder          string
	OnlyOkteto       bool
	Show             bool
	Save             bool
	IsCtxCommand     bool
	IsOkteto         bool
	raiseNotCtxError bool
}

func (o *ContextOptions) initFromContext() {
	if o.IsCtxCommand {
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
		o.IsOkteto = okCtx.IsOkteto
	}
}

func (o *ContextOptions) initFromEnvVars() {
	if o.Token == "" {
		o.Token = os.Getenv(model.OktetoTokenEnvVar)
	}

	if o.Context == "" && os.Getenv(model.OktetoURLEnvVar) != "" {
		o.Context = os.Getenv(model.OktetoURLEnvVar)
		o.IsOkteto = true
	}

	if o.Context == "" && os.Getenv(model.OktetoContextEnvVar) != "" {
		o.Context = os.Getenv(model.OktetoContextEnvVar)
	}

	if o.Token != "" {
		o.IsOkteto = true
		if o.Context == "" {
			o.Context = okteto.CloudURL
		}
	}

	if o.Namespace == "" && os.Getenv(model.OktetoNamespaceEnvVar) != "" {
		o.Namespace = os.Getenv(model.OktetoNamespaceEnvVar)
	}
}
