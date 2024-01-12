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

package pipeline

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/devenvironment"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/repository"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
)

var (
	errNoK8sCtxAvailable = errors.New("no kubernetes context available")
)

// Options are the deploy options for the pipeline command
type Options struct {
	fs                afero.Fs
	k8sClientProvider k8sClientProvider
	okCtxController   okCtxController
	ioCtrl            *io.IOController
	okClient          types.OktetoInterface
	projectRootPath   string
	Branch            string
	Repository        string
	Name              string
	Namespace         string
	File              string
	Variables         []string
	Labels            []string
	Timeout           time.Duration
	Wait              bool
	SkipIfExists      bool
	ReuseParams       bool
}

// oktetoClientProvider provides an okteto client ready to use or fail
type oktetoClientProvider interface {
	Provide(...okteto.Option) (types.OktetoInterface, error)
}

// OptFunc sets options such as branch, repository, etc.
type OptFunc func(*Options)

// WithBranch sets the branch to deploy
func WithBranch(branch string) OptFunc {
	return func(o *Options) {
		o.Branch = branch
	}
}

// WithRepository sets the repository to deploy
func WithRepository(repository string) OptFunc {
	return func(o *Options) {
		o.Repository = repository
	}
}

// WithName sets the name of the pipeline
func WithName(name string) OptFunc {
	return func(o *Options) {
		o.Name = name
	}
}

// WithNamespace sets the namespace of the pipeline
func WithNamespace(namespace string) OptFunc {
	return func(o *Options) {
		o.Namespace = namespace
	}
}

// WithFile sets the file of the pipeline
func WithFile(file string) OptFunc {
	return func(o *Options) {
		o.File = file
	}
}

// WithVariables sets the variables of the pipeline
func WithVariables(variables []string) OptFunc {
	return func(o *Options) {
		o.Variables = variables
	}
}

// WithLabels sets the labels of the pipeline
func WithLabels(labels []string) OptFunc {
	return func(o *Options) {
		o.Labels = labels
	}
}

// WithSkipIfExists sets the skipifexists of the pipeline
func WithSkipIfExists(skipIfExists bool) OptFunc {
	return func(o *Options) {
		o.SkipIfExists = skipIfExists
	}
}

// WithSkipIfExists sets the skipifexists of the pipeline
func WithReuseParams(reusearams bool) OptFunc {
	return func(o *Options) {
		o.ReuseParams = reusearams
	}
}

// WithTimeout sets the timeout of the pipeline
func WithTimeout(timeout time.Duration) OptFunc {
	return func(o *Options) {
		o.Timeout = timeout
	}
}

// WithWait sets the wait of the pipeline
func WithWait(wait bool) OptFunc {
	return func(o *Options) {
		o.Wait = wait
	}
}

func WithK8sProvider(provider k8sClientProvider) OptFunc {
	return func(o *Options) {
		o.k8sClientProvider = provider
	}
}

func WithFs(fs afero.Fs) OptFunc {
	return func(o *Options) {
		o.fs = fs
	}
}

func WithIO(ioCtrl *io.IOController) OptFunc {
	return func(o *Options) {
		o.ioCtrl = ioCtrl
	}
}

func WithOkCtxController(okCtxController okCtxController) OptFunc {
	return func(o *Options) {
		o.okCtxController = okCtxController
	}
}

func WithProjectRootPath(projectRootPath string) OptFunc {
	return func(o *Options) {
		o.projectRootPath = projectRootPath
	}
}

func WithOktetoAPICtrl(okClientProvider oktetoClientProvider) OptFunc {
	return func(o *Options) {
		okClient, err := okClientProvider.Provide()
		if err != nil && o.ioCtrl != nil {
			o.ioCtrl.Debugf("error getting okteto client: %s", err)
			return
		}
		o.okClient = okClient
	}
}

func (o *Options) setDefaults() error {
	if err := o.setDefaultRepository(); err != nil {
		return fmt.Errorf("could not set default repository: %w", err)
	}

	o.setDefaultNamespace()

	if err := o.setDefaultName(); err != nil {
		return fmt.Errorf("could not set default name: %w", err)
	}

	if err := o.setDefaultBranch(); err != nil {
		return fmt.Errorf("could not set default branch: %w", err)
	}

	return nil
}

func (o *Options) setDefaultRepository() error {
	if o.Repository == "" {
		o.ioCtrl.Logger().Info("inferring git repository URL")
		var err error
		o.Repository, err = model.GetRepositoryURL(o.projectRootPath)
		if err != nil {
			return fmt.Errorf("could not get repository url: %w", err)
		}
	}
	return nil
}

func (o *Options) setDefaultNamespace() error {
	if o.Namespace == "" {
		o.ioCtrl.Logger().Info("inferring namespace")
		o.Namespace = o.okCtxController.GetNamespace()
	}
	return nil
}

func (o *Options) setDefaultName() error {
	if o.Name == "" {
		if o.okCtxController.GetK8sConfig() == nil || o.k8sClientProvider == nil {
			return fmt.Errorf("could not infer name from repository: %w", errNoK8sCtxAvailable)
		}

		// in case of inferring the name, o.Name is not sanitized
		c, _, err := o.k8sClientProvider.Provide(o.okCtxController.GetK8sConfig())
		if err != nil {
			return fmt.Errorf("could not infer name from repository: %w", err)
		}
		inferer := devenvironment.NewNameInferer(c)
		o.Name = inferer.InferNameFromDevEnvsAndRepository(context.Background(), o.Repository, o.Namespace, o.File, "")
	}
	return nil
}

func (o *Options) setDefaultBranch() error {
	currentRepoURL, err := model.GetRepositoryURL(o.projectRootPath)
	if err != nil {
		o.ioCtrl.Debug("cwd does not have .git folder")
	}

	currentRepo := repository.NewRepository(currentRepoURL)
	optsRepo := repository.NewRepository(o.Repository)
	if o.Branch == "" && currentRepo.IsEqual(optsRepo) {
		o.ioCtrl.Logger().Info("inferring branch")
		var err error
		o.Branch, err = currentRepo.GetBranch()
		if err != nil {
			return fmt.Errorf("could not get branch: %w", err)
		}
	}
	return nil
}

func (o *Options) SetFromCmap(cmap *corev1.ConfigMap) {
	if cmap == nil || cmap.Data == nil {
		return
	}

	if v, ok := cmap.Data["name"]; ok {
		o.Name = v
	}
	if v, ok := cmap.Data["filename"]; ok {
		o.File = v
	}
	if v, ok := cmap.Data["repository"]; ok {
		o.Repository = v
	}
	if v, ok := cmap.Data["branch"]; ok {
		o.Branch = v
	}
	if v, ok := cmap.Data["variables"]; ok {
		vars, err := parseVariablesListFromCfgVariablesString(v)
		if err != nil {
			o.ioCtrl.Debugf("error parsing variables %v", err)
		}
		o.Variables = vars
	}
	if l := parseEnvironmentLabelFromLabelsMap(cmap.Labels); l != nil {
		o.Labels = l
	}
	o.Namespace = cmap.Namespace
}

// parseVariablesListFromCfgVariablesString returns a list of strings with the variables decoded from the configmap
func parseVariablesListFromCfgVariablesString(vEncodedString string) ([]string, error) {
	if len(vEncodedString) == 0 {
		return nil, nil
	}

	vListString := make([]string, 0)
	b, err := base64.StdEncoding.DecodeString(vEncodedString)
	if err != nil {
		return nil, err
	}

	var vars []types.Variable
	if err := json.Unmarshal(b, &vars); err != nil {
		return nil, err
	}
	for _, v := range vars {
		vString := fmt.Sprintf("%s=%s", v.Name, v.Value)
		vListString = append(vListString, vString)
	}
	return vListString, nil
}

// parseEnvironmentLabelFromLabelsMap returns a list of strings with the environment labels from the configmap
func parseEnvironmentLabelFromLabelsMap(labels map[string]string) []string {
	if len(labels) == 0 {
		return nil
	}

	l := make([]string, 0)
	for k := range labels {
		if strings.HasPrefix(k, constants.EnvironmentLabelKeyPrefix) {
			_, label, found := strings.Cut(k, "/")
			if !found || label == "" {
				continue
			}
			l = append(l, label)
		}
	}
	return l
}

func (o *Options) toPipelineDeployClientOptions() (types.PipelineDeployOptions, error) {
	var varList []types.Variable
	for _, v := range o.Variables {
		variableFormatParts := 2
		kv := strings.SplitN(v, "=", variableFormatParts)
		if len(kv) != variableFormatParts {
			return types.PipelineDeployOptions{}, fmt.Errorf("invalid variable value '%s': must follow KEY=VALUE format", v)
		}
		varList = append(varList, types.Variable{
			Name:  kv[0],
			Value: kv[1],
		})
	}
	return types.PipelineDeployOptions{
		Name:       o.Name,
		Repository: o.Repository,
		Branch:     o.Branch,
		Filename:   o.File,
		Variables:  varList,
		Namespace:  o.Namespace,
		Labels:     o.Labels,
	}, nil
}
