package deploy

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestBuildImages(t *testing.T) {
	testCases := []struct {
		expectedError        error
		builder              *fakeV2Builder
		stack                *model.Stack
		name                 string
		buildServices        []string
		servicesToDeploy     []string
		servicesAlreadyBuilt []string
		expectedImages       []string
		build                bool
	}{
		{
			name: "everything",
			builder: &fakeV2Builder{
				servicesAlreadyBuilt: []string{"manifest B", "stack A"},
			},
			build:         false,
			buildServices: []string{"manifest A", "manifest B", "stack A", "stack B"},
			stack: &model.Stack{Services: map[string]*model.Service{
				"stack A":             {Build: &build.Info{}},
				"stack B":             {Build: &build.Info{}},
				"stack without build": {},
			}},
			servicesToDeploy: []string{"stack A", "stack without build"},
			expectedError:    nil,
			expectedImages:   []string{"manifest A"},
		},
		{
			name:             "nil stack",
			builder:          &fakeV2Builder{},
			build:            false,
			buildServices:    []string{"manifest A", "manifest B"},
			stack:            nil,
			servicesToDeploy: []string{"manifest A"},
			expectedError:    nil,
			expectedImages:   []string{"manifest A", "manifest B"},
		},
		{
			name: "no services to deploy",
			builder: &fakeV2Builder{
				servicesAlreadyBuilt: []string{"stack"},
			},
			build:         false,
			buildServices: []string{"manifest", "stack"},
			stack: &model.Stack{Services: map[string]*model.Service{
				"stack": {Build: &build.Info{}},
			}},
			servicesToDeploy: []string{},
			expectedError:    nil,
			expectedImages:   []string{"manifest"},
		},
		{
			name:          "no services already built",
			builder:       &fakeV2Builder{},
			build:         false,
			buildServices: []string{"manifest A", "stack B", "stack C"},
			stack: &model.Stack{Services: map[string]*model.Service{
				"stack B": {Build: &build.Info{}},
				"stack C": {Build: &build.Info{}},
			}},
			servicesToDeploy: []string{"manifest A", "stack C"},
			expectedError:    nil,
			expectedImages:   []string{"manifest A", "stack C"},
		},
		{
			name: "force build",
			builder: &fakeV2Builder{
				servicesAlreadyBuilt: []string{"should be ignored since build is forced", "manifest A", "stack B"},
			},
			build:         true,
			buildServices: []string{"manifest A", "manifest B", "stack A", "stack B"},
			stack: &model.Stack{Services: map[string]*model.Service{
				"stack A": {Build: &build.Info{}},
				"stack B": {Build: &build.Info{}},
			}},
			servicesToDeploy: []string{"stack A", "stack B"},
			expectedError:    nil,
			expectedImages:   []string{"manifest A", "manifest B", "stack A", "stack B"},
		},
		{
			name: "force build specific services",
			builder: &fakeV2Builder{
				servicesAlreadyBuilt: []string{"should be ignored since build is forced", "manifest A", "stack B"},
			},
			build:         true,
			buildServices: []string{"manifest A", "manifest B", "stack A", "stack B"},
			stack: &model.Stack{Services: map[string]*model.Service{
				"stack A":             {Build: &build.Info{}},
				"stack B":             {Build: &build.Info{}},
				"stack without build": {},
			}},
			servicesToDeploy: []string{"stack A", "stack without build"},
			expectedError:    nil,
			expectedImages:   []string{"manifest A", "manifest B", "stack A"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			deployOptions := &Options{
				Build: testCase.build,
				Manifest: &model.Manifest{
					Build: build.ManifestBuild{},
					Deploy: &model.DeployInfo{
						ComposeSection: &model.ComposeSectionInfo{
							Stack: testCase.stack,
						},
					},
				},
				servicesToDeploy: testCase.servicesToDeploy,
			}

			for _, service := range testCase.buildServices {
				deployOptions.Manifest.Build[service] = &build.Info{}
			}

			err := buildImages(context.Background(), testCase.builder, deployOptions)
			assert.Equal(t, testCase.expectedError, err)
			assert.Equal(t, sliceToSet(testCase.expectedImages), sliceToSet(testCase.builder.buildOptionsStorage.CommandArgs))
		})
	}

}
