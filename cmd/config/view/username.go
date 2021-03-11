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

package view

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

//Username returns the username of the authenticated user
func Username(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "username",
		Short: "Returns the username of the authenticated user",
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := okteto.GetToken()
			if err != nil {
				log.Infof("error getting okteto token: %s", err.Error())
				return errors.ErrNotLogged
			}
			if t.Username != "" {
				fmt.Println(t.Username)
				return nil
			}
			log.Info("refreshing okteto token...")
			u, err := login.WithToken(ctx, t.URL, t.Token)
			if err != nil {
				log.Infof("error refreshing okteto token: %s", err.Error())
				return errors.ErrNotLogged
			}
			fmt.Println(u.ExternalID)
			return nil
		},
	}

	return cmd
}
