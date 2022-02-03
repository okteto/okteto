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

package test

import (
	"context"

	"github.com/okteto/okteto/pkg/types"
)

type FakeLoginController struct {
	User *types.User
	Err  error
}

func NewFakeLoginController(user *types.User, err error) *FakeLoginController {
	return &FakeLoginController{User: user, Err: err}
}

func (fakeController FakeLoginController) AuthenticateToOktetoCluster(ctx context.Context, oktetoURL, token string) (*types.User, error) {
	return fakeController.User, fakeController.Err
}
