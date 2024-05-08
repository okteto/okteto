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

package client

import (
	"context"

	dockertypes "github.com/docker/cli/cli/config/types"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/types"
)

// FakeUserClient is used to mock the userClient interface
type FakeUserClient struct {
	errGetPlatformVariables error
	userCtx                 *types.UserContext
	platformVariables       []env.Var
	err                     []error
	ClusterMetadata         types.ClusterMetadata
}

func NewFakeUsersClient(user *types.User, err ...error) *FakeUserClient {
	return &FakeUserClient{userCtx: &types.UserContext{User: *user}, err: err}
}

func NewFakeUsersClientWithContext(userCtx *types.UserContext, err ...error) *FakeUserClient {
	return &FakeUserClient{userCtx: userCtx, err: err}
}

func (c *FakeUserClient) GetContext(_ context.Context, _ string) (*types.UserContext, error) {
	if c.err != nil && len(c.err) > 0 {
		err := c.err[0]
		c.err = c.err[1:]
		if err != nil {
			return nil, err
		}
	}

	return c.userCtx, nil
}

func (c *FakeUserClient) GetOktetoPlatformVariables(_ context.Context) ([]env.Var, error) {
	if c.errGetPlatformVariables != nil {
		return nil, c.errGetPlatformVariables
	}
	return c.platformVariables, nil
}

func (*FakeUserClient) GetClusterCertificate(_ context.Context, _, _ string) ([]byte, error) {
	return nil, nil
}

func (c *FakeUserClient) GetClusterMetadata(_ context.Context, _ string) (types.ClusterMetadata, error) {
	if len(c.err) > 0 {
		return types.ClusterMetadata{}, c.err[0]
	}
	return c.ClusterMetadata, nil
}

func (*FakeUserClient) GetRegistryCredentials(_ context.Context, _ string) (dockertypes.AuthConfig, error) {
	return dockertypes.AuthConfig{}, nil
}
