//go:build integration
// +build integration

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

package kubetoken

import (
	"encoding/json"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/require"
)

// TestKubetokenHasExpiration test that kubernetes returns a dynamic token with a set expiration
func TestKubetokenHasExpiration(t *testing.T) {
	t.Parallel()

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	out, err := commands.RunOktetoKubetoken(oktetoPath, "")
	require.NoError(t, err)

	var resp *types.KubeTokenResponse
	err = json.Unmarshal(out.Bytes(), &resp)
	require.NoError(t, err)

	parser := new(jwt.Parser)
	token, _, err := parser.ParseUnverified(resp.Status.Token, jwt.MapClaims{})
	require.NoError(t, err)

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if _, ok := claims["exp"]; !ok {
			t.Fatal("Expiration claim is not set")
		}
	} else {
		t.Fatal("Invalid claims")
	}
}
