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

package preview

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	okErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Deploy Create and deploy a preview environment
func Deploy(ctx context.Context) *cobra.Command {
	var branch string
	var filename string
	var repository string
	var scope string
	var sourceUrl string
	var variables []string

	cmd := &cobra.Command{
		Use:   "deploy <name>",
		Short: "Create and deploy a preview environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			if !okteto.IsAuthenticated() {
				return okErrors.ErrNotLogged
			}

			if err := validatePreviewType(scope); err != nil {
				return err
			}

			var err error
			repository, err = getRepository(ctx, repository)
			if err != nil {
				return err
			}
			branch, err = getBranch(ctx, branch)
			if err != nil {
				return err
			}

			name := ""
			if len(args) == 0 {
				name, err = getName(ctx, repository, branch)
				if err != nil {
					return err
				}
			} else {
				name = args[0]
			}

			varList := []okteto.Variable{}
			for _, v := range variables {
				kv := strings.SplitN(v, "=", 2)
				if len(kv) != 2 {
					return fmt.Errorf("invalid variable value '%s': must follow KEY=VALUE format", v)
				}
				varList = append(varList, okteto.Variable{
					Name:  kv[0],
					Value: kv[1],
				})
			}

			err = executeDeployPreview(ctx, name, scope, repository, branch, sourceUrl, filename, varList)
			analytics.TrackCreatePreview(err == nil)
			return err
		},
	}
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "the branch to deploy (defaults to the current branch)")
	cmd.Flags().StringVarP(&filename, "filename", "f", "", "relative path within the repository to the manifest file (default to okteto-pipeline.yaml or .okteto/okteto-pipeline.yaml)")
	cmd.Flags().StringVarP(&repository, "repository", "r", "", "the repository to deploy (defaults to the current repository)")
	cmd.Flags().StringVarP(&scope, "scope", "s", "personal", "the scope of preview environment to create. Accepted values are ['personal', 'global']")
	cmd.Flags().StringVarP(&sourceUrl, "sourceUrl", "", "", "the pull request url to notify.")
	cmd.Flags().StringArrayVarP(&variables, "var", "v", []string{}, "set a pipeline variable (can be set more than once)")

	return cmd
}

func validatePreviewType(previewType string) error {
	if !(previewType == "global" || previewType == "personal") {
		return fmt.Errorf("Value '%s' is invalid for flag 'type'. Accepted values are ['global', 'personal']", previewType)
	}
	return nil
}

func getRepository(ctx context.Context, repository string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get the current working directory: %w", err)
	}

	if repository == "" {
		log.Info("inferring git repository URL")

		r, err := model.GetRepositoryURL(cwd)

		if err != nil {
			return "", err
		}

		repository = r
	}
	return repository, nil
}

func getBranch(ctx context.Context, branch string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get the current working directory: %w", err)
	}
	if branch == "" {
		log.Info("inferring git repository branch")
		b, err := utils.GetBranch(ctx, cwd)

		if err != nil {
			return "", err
		}
		branch = b
	}
	return branch, nil
}

func getName(ctx context.Context, repo, branch string) (string, error) {
	username := okteto.GetUsername()

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get the current working directory: %w", err)
	}
	repo, err = model.GetValidNameFromGitRepo(cwd)
	if err != nil {
		return "", err
	}
	branch = model.ValidKubeNameRegex.ReplaceAllString(branch, "-")
	name := strings.ToLower(fmt.Sprintf("%s-%s-%s", repo, branch, username))

	return name, nil
}

func executeDeployPreview(ctx context.Context, name, scope, repository, branch, sourceUrl, filename string, variables []okteto.Variable) error {
	oktetoNS, err := okteto.DeployPreview(ctx, name, scope, repository, branch, sourceUrl, filename, variables)
	if err != nil {
		return err
	}

	log.Success("Preview environment '%s' created", oktetoNS)

	return nil
}
