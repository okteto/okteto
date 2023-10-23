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

package up

import (
	"context"
	"os/exec"
	"time"

	"github.com/moby/term"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	apiv1 "k8s.io/api/core/v1"
)

type registryInterface interface {
	GetImageTagWithDigest(imageTag string) (string, error)
	GetImageTag(image, service, namespace string) string
}

type builderInterface interface {
	GetServicesToBuild(ctx context.Context, manifest *model.Manifest, svcToDeploy []string) ([]string, error)
	Build(ctx context.Context, options *types.BuildOptions) error
	GetBuildEnvVars() map[string]string
}

type analyticsTrackerInterface interface {
	TrackImageBuild(...*analytics.ImageBuildMetadata)
	TrackDeploy(analytics.DeployMetadata)
	TrackUp(*analytics.UpMetricsMetadata)
}

// upContext is the common context of all operations performed during the up command
type upContext struct {
	Cancel                context.CancelFunc
	Registry              registryInterface
	ShutdownCompleted     chan bool
	Manifest              *model.Manifest
	Dev                   *model.Dev
	Translations          map[string]*apps.Translation
	isRetry               bool
	Pod                   *apiv1.Pod
	Forwarder             forwarder
	Disconnect            chan error
	GlobalForwarderStatus chan error
	CommandResult         chan error
	Exit                  chan error
	Sy                    *syncthing.Syncthing
	cleaned               chan string
	hardTerminate         chan error
	success               bool
	resetSyncthing        bool
	inFd                  uintptr
	isTerm                bool
	stateTerm             *term.State
	StartTime             time.Time
	Options               *UpOptions
	pidController         pidController
	tokenUpdater          tokenUpdater
	K8sClientProvider     okteto.K8sClientProvider
	Fs                    afero.Fs
	hybridCommand         *exec.Cmd
	interruptReceived     bool
	analyticsTracker      analyticsTrackerInterface
	analyticsMeta         *analytics.UpMetricsMetadata
	builder               builderInterface
}

// Forwarder is an interface for the port-forwarding features
type forwarder interface {
	Add(forward.Forward) error
	AddReverse(model.Reverse) error
	Start(string, string) error
	StartGlobalForwarding() error
	Stop()
	TransformLabelsToServiceName(forward.Forward) (forward.Forward, error)
}
