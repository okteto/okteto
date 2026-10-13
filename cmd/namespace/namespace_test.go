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

package namespace

import (
	"context"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/types"
)

type fakeWakeAnalyticsTracker struct {
	calls []analytics.WakeTriggeredMetadata
}

func (f *fakeWakeAnalyticsTracker) TrackWakeTriggered(_ context.Context, m analytics.WakeTriggeredMetadata) {
	f.calls = append(f.calls, m)
}

func newFakeContextCommand(c *client.FakeOktetoClient, user *types.User) *contextCMD.Command {
	cmd := contextCMD.NewContextCommand()
	cmd.OktetoClientProvider = client.NewFakeOktetoClientProvider(c)
	cmd.K8sClientProvider = test.NewFakeK8sProvider(nil)
	cmd.LoginController = test.NewFakeLoginController(user, nil)
	cmd.OktetoContextWriter = test.NewFakeOktetoContextWriter()
	return cmd
}

func NewFakeNamespaceCommand(okClient *client.FakeOktetoClient, user *types.User) *Command {
	return &Command{
		okClient: okClient,
		ctxCmd:   newFakeContextCommand(okClient, user),
	}
}
