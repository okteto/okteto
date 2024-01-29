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

package context

import (
	"os"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

type Options struct {
	Token                 string
	Context               string
	Namespace             string
	Builder               string
	OnlyOkteto            bool
	Show                  bool
	Save                  bool
	SetCurrentNs          bool
	IsCtxCommand          bool
	CheckNamespaceAccess  bool
	IsOkteto              bool
	raiseNotCtxError      bool
	InsecureSkipTlsVerify bool
	InferredToken         bool
}

func (o *Options) InitFromContext() {
	if o.IsCtxCommand {
		return
	}
	if o.Context != "" {
		return
	}
	ctxStore := okteto.GetContextStore()
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

func (o *Options) InitFromEnvVars() {
	usedEnvVars := []string{}

	if o.Context == "" && os.Getenv(model.OktetoURLEnvVar) != "" {
		o.Context = os.Getenv(model.OktetoURLEnvVar)
		o.IsOkteto = true
		usedEnvVars = append(usedEnvVars, model.OktetoURLEnvVar)
	}

	if o.Context == "" && os.Getenv(model.OktetoContextEnvVar) != "" {
		o.Context = os.Getenv(model.OktetoContextEnvVar)
		usedEnvVars = append(usedEnvVars, model.OktetoContextEnvVar)
	}

	envToken := os.Getenv(model.OktetoTokenEnvVar)
	if o.Token != "" || envToken != "" {
		o.IsOkteto = true
		if o.Context == "" {
			o.Context = okteto.CloudURL
		}
	}

	if o.Token == "" && envToken != "" {
		if !okteto.HasBeenLogged(o.Context) || okteto.GetContext().Token != envToken {
			usedEnvVars = append(usedEnvVars, model.OktetoTokenEnvVar)
		}
		o.Token = envToken
		o.InferredToken = true
	}

	if o.Namespace == "" && os.Getenv(model.OktetoNamespaceEnvVar) != "" {
		o.Namespace = os.Getenv(model.OktetoNamespaceEnvVar)
		usedEnvVars = append(usedEnvVars, model.OktetoNamespaceEnvVar)
	}

	if len(usedEnvVars) == 1 {
		oktetoLog.Warning("Initializing context with the value of %s environment variable", usedEnvVars[0])
	} else if len(usedEnvVars) > 1 {
		oktetoLog.Warning("Initializing context with the value of %s and %s environment variables", strings.Join(usedEnvVars[0:len(usedEnvVars)-1], ", "), usedEnvVars[len(usedEnvVars)-1])
	}

}
