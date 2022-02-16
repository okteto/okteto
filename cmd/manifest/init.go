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

package manifest

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/cmd/utils/executor"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// ManifestCommand has all the namespaces subcommands
type ManifestCommand struct {
	manifest *model.Manifest
}

// InitOpts defines the option for manifest init
type InitOpts struct {
	DevPath   string
	Namespace string
	Context   string
	Overwrite bool

	ShowCTA bool
}

// Init automatically generates the manifest
func Init() *cobra.Command {
	opts := &InitOpts{}
	cmd := &cobra.Command{
		Use:   "init",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#init"),
		Short: "Automatically generate your okteto manifest file",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			ctxResource := &model.ContextResource{}
			if err := ctxResource.UpdateNamespace(opts.Namespace); err != nil {
				return err
			}

			if err := ctxResource.UpdateContext(opts.Context); err != nil {
				return err
			}
			ctxOptions := &contextCMD.ContextOptions{
				Context:   ctxResource.Context,
				Namespace: ctxResource.Namespace,
				Show:      true,
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOptions); err != nil {
				return err
			}

			opts.ShowCTA = oktetoLog.IsInteractive()
			mc := &ManifestCommand{}

			return mc.Init(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "", "namespace target for generating the okteto manifest")
	cmd.Flags().StringVarP(&opts.Context, "context", "c", "", "context target for generating the okteto manifest")
	cmd.Flags().StringVarP(&opts.DevPath, "file", "f", utils.DefaultManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&opts.Overwrite, "replace", "r", false, "overwrite existing manifest file")
	return cmd
}

// Init initializes a new okteto manifest
func (mc *ManifestCommand) Init(ctx context.Context, opts *InitOpts) error {
	if err := validateDevPath(opts.DevPath, opts.Overwrite); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	composeFiles := utils.GetStackFiles(cwd)

	var manifest *model.Manifest
	if len(composeFiles) > 0 {
		composePath, create, err := selectComposeFile(composeFiles)
		if err != nil {
			return err
		}
		if composePath != "" {
			answer, err := utils.AskYesNo("creating an okteto manifest is optional if you want to use a compose file. Do you want to continue? ")
			if err != nil {
				return err
			}
			if !answer {
				return nil
			}
			manifest, err = createFromCompose(composePath)
			if err != nil {
				return err
			}
		} else if create {
			manifest, err = createNewCompose()
			if err != nil {
				return err
			}
		} else {
			manifest, err = createFromKubernetes(cwd)
			if err != nil {
				return err
			}
		}
	} else {
		selection, err := utils.AskForOptions([]string{"compose", "kubernetes"}, "How do you want to deploy your application?")
		if err != nil {
			return err
		}
		if selection == "compose" {
			manifest, err = createNewCompose()
			if err != nil {
				return err
			}
		} else {
			manifest, err = createFromKubernetes(cwd)
			if err != nil {
				return err
			}
		}
	}

	if manifest != nil {
		mc.manifest = manifest
		manifest.Name = utils.InferName(cwd)
		if err := manifest.WriteToFile(opts.DevPath); err != nil {
			return err
		}
		if opts.ShowCTA {
			answer, err := utils.AskYesNo("Do you want to deploy?")
			if err != nil {
				return err
			}
			if answer {
				kubeconfig := deploy.NewKubeConfig()
				proxy, err := deploy.NewProxy(manifest.Name, kubeconfig)
				if err != nil {
					return err
				}

				c := &deploy.DeployCommand{
					GetManifest:        mc.getManifest,
					Kubeconfig:         kubeconfig,
					Executor:           executor.NewExecutor(oktetoLog.GetOutputFormat()),
					Proxy:              proxy,
					TempKubeconfigFile: deploy.GetTempKubeConfigFile(manifest.Name),
					K8sClientProvider:  okteto.NewK8sClientProvider(),
				}

				return c.RunDeploy(ctx, &deploy.Options{
					Name:         manifest.Name,
					ManifestPath: opts.DevPath,
					Timeout:      5 * time.Minute,
					Build:        true,
				})
			}
			oktetoLog.Success("Okteto manifest configured correctly.\n    Run `okteto up` to activate your development container")
		}
	}
	return nil
}

func validateDevPath(devPath string, overwrite bool) error {
	if !overwrite && model.FileExists(devPath) {
		return fmt.Errorf("%s already exists. Run this command again with the '--replace' flag to overwrite it", devPath)
	}
	return nil
}

func createFromCompose(composePath string) (*model.Manifest, error) {
	stack, err := model.LoadStack("", []string{composePath})
	if err != nil {
		return nil, err
	}
	manifest := &model.Manifest{
		Type: model.StackType,
		Deploy: &model.DeployInfo{
			Compose: &model.ComposeInfo{
				Manifest: []string{
					composePath,
				},
				Stack: stack,
			},
		},
		Dev:      model.ManifestDevs{},
		Build:    model.ManifestBuild{},
		Filename: composePath,
		IsV2:     true,
	}
	manifest, err = manifest.InferFromStack()
	if err != nil {
		return nil, err
	}
	manifest.Context = okteto.Context().Name
	manifest.Namespace = okteto.Context().Namespace

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get the current working directory: %w", err)
	}
	for _, build := range manifest.Build {
		build.Context, err = filepath.Rel(cwd, build.Context)
		if err != nil {
			return nil, fmt.Errorf("can not set the relative path of '%s' from your current working directory: '%s'", build.Context, cwd)
		}
		build.Dockerfile, err = filepath.Rel(cwd, build.Dockerfile)
		if err != nil {
			return nil, fmt.Errorf("can not set the relative path of '%s' from your current working directory: '%s'", build.Context, cwd)
		}
	}
	return manifest, err
}

func createNewCompose() (*model.Manifest, error) {
	oktetoLog.Information("docker-compose helps you define a multicontainer application. Learn how to configure a compose here: https://github.com/compose-spec/compose-spec/blob/master/spec.md")
	return nil, nil
}

func createFromKubernetes(cwd string) (*model.Manifest, error) {
	manifest := model.NewManifest()
	dockerfiles, err := selectDockerfiles(cwd)
	if err != nil {
		return nil, err
	}
	manifest.Build, err = inferBuildSectionFromDockerfiles(cwd, dockerfiles)
	if err != nil {
		return nil, err
	}
	manifest.Deploy, err = inferDeploySection(cwd)
	if err != nil {
		return nil, err
	}
	manifest.Dev, err = inferDevsSection(cwd)

	return manifest, nil
}

func inferBuildSectionFromDockerfiles(cwd string, dockerfiles []string) (model.ManifestBuild, error) {
	manifestBuild := model.ManifestBuild{}
	for _, dockerfile := range dockerfiles {
		var name string
		var buildInfo *model.BuildInfo
		if dockerfile == dockerfileName {
			name = utils.InferName(cwd)
			buildInfo = &model.BuildInfo{
				Context: ".",
			}
		} else {
			name = filepath.Dir(dockerfile)
			buildInfo = &model.BuildInfo{
				Context: filepath.Dir(dockerfile),
			}
		}
		if !okteto.IsOkteto() {
			imageName, err := utils.AsksQuestion(fmt.Sprintf("Which is the image name for %s", dockerfile))
			if err != nil {
				return nil, err
			}
			buildInfo.Image = imageName
		}
		manifestBuild[name] = buildInfo
	}
	return manifestBuild, nil
}

func inferDeploySection(cwd string) (*model.DeployInfo, error) {
	m, err := model.GetInferredManifest(cwd)
	if err != nil {
		return nil, err
	}
	if m != nil && m.Deploy != nil {
		return m.Deploy, nil
	}
	return &model.DeployInfo{
		Commands: []model.DeployCommand{
			{
				Name:    "Replace this line with helm or kubectl commands to deploy your application",
				Command: "Replace this line with helm or kubectl commands to deploy your application",
			},
		},
	}, nil
}

func inferDevsSection(cwd string) (model.ManifestDevs, error) {
	files, err := ioutil.ReadDir(cwd)
	if err != nil {
		return nil, err
	}
	devs := model.ManifestDevs{}
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		dev, err := model.GetManifestV2(f.Name())
		if err != nil {
			oktetoLog.Debugf("could not detect any okteto manifest on %s", f.Name())
			continue
		}
		if dev.IsV2 == false && len(dev.Dev) != 0 {
			devs[f.Name()] = dev.Dev[f.Name()]
		}
	}
	return devs, nil
}

func (mc *ManifestCommand) getManifest(path string) (*model.Manifest, error) {
	if mc.manifest != nil {
		return mc.manifest, nil
	}
	return model.GetManifestV2(path)
}
