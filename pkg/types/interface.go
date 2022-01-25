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

import "context"

type OktetoInterface interface {
	User() UserInterface
	Namespaces() NamespaceInterface
	Previews() PreviewInterface
}

type UserInterface interface {
	GetContext(ctx context.Context) (*UserContext, error)
}

type NamespaceInterface interface {
	Create(ctx context.Context, namespace string) (string, error)
	List(ctx context.Context) ([]Namespace, error)
	Delete(ctx context.Context, namespace string) error
	AddMembers(ctx context.Context, namespace string, members []string) error
	SleepNamespace(ctx context.Context, namespace string) error
}

type PreviewInterface interface {
	List(ctx context.Context) ([]Preview, error)
}

type OktetoClientProvider interface {
	Provide() (OktetoInterface, error)
}
