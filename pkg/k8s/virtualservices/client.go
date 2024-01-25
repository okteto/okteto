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

package virtualservices

import (
	"github.com/okteto/okteto/pkg/okteto"
	istioclientset "istio.io/client-go/pkg/clientset/versioned"
)

// GetIstioClient returns a client for istio
func GetIstioClient() (*istioclientset.Clientset, error) {
	_, config, err := okteto.NewK8sClientProvider().Provide(okteto.GetContext().Cfg)
	if err != nil {
		return nil, err
	}
	return istioclientset.NewForConfig(config)
}
