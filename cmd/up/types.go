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

package up

import (
	"context"
	"time"

	"github.com/moby/term"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/syncthing"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// upContext is the common context of all operations performed during the up command
type upContext struct {
	Cancel            context.CancelFunc
	ShutdownCompleted chan bool
	Dev               *model.Dev
	isOktetoNamespace bool
	isSwap            bool
	isRetry           bool
	Client            *kubernetes.Clientset
	RestConfig        *rest.Config
	Pod               *apiv1.Pod
	Forwarder         forwarder
	Disconnect        chan error
	CommandResult     chan error
	Exit              chan error
	Sy                *syncthing.Syncthing
	cleaned           chan string
	hardTerminate     chan error
	success           bool
	resetSyncthing    bool
	inFd              uintptr
	isTerm            bool
	stateTerm         *term.State
	spinner           *utils.Spinner
	StartTime         time.Time
}

// Forwarder is an interface for the port-forwarding features
type forwarder interface {
	Add(model.Forward) error
	AddReverse(model.Reverse) error
	Start(string, string) error
	Stop()
	TransformLabelsToServiceName(model.Forward) (model.Forward, error)
}
