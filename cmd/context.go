package cmd

// Copyright 2020 The Okteto Authors
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

import (
	"context"

	"github.com/okteto/okteto/pkg/k8s/forward"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/syncthing"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// UpContext is the common context of all operations performed during
// the up command
type UpContext struct {
	Context    context.Context
	Cancel     context.CancelFunc
	Dev        *model.Dev
	Namespace  *apiv1.Namespace
	isSwap     bool
	retry      bool
	Client     *kubernetes.Clientset
	RestConfig *rest.Config
	Pod        string
	Forwarder  *forward.PortForwardManager
	Disconnect chan struct{}
	Running    chan error
	Exit       chan error
	Sy         *syncthing.Syncthing
	ErrChan    chan error
	cleaned    chan struct{}
	success    bool
}


