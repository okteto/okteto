package test

import (
	"context"
	"fmt"
	"testing"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

type fakeSvcFromRegex struct {
	err error
	svc string
}

type fakeSvcToBuildDuringDeploy struct {
	err  error
	svcs []string
}

type fakeBuilderV2 struct {
	svcFromRegex           fakeSvcFromRegex
	svcToBuildDuringDeploy fakeSvcToBuildDuringDeploy
	build                  error
}

func (fb *fakeBuilderV2) GetSvcToBuildFromRegex(ctx context.Context, manifest *model.Manifest, imgFinder model.ImageFromManifest) (string, error) {
	return fb.svcFromRegex.svc, fb.svcFromRegex.err
}
func (fb *fakeBuilderV2) GetServicesToBuildDuringDeploy(ctx context.Context, manifest *model.Manifest, svcsToDeploy []string) ([]string, error) {
	return fb.svcToBuildDuringDeploy.svcs, fb.svcToBuildDuringDeploy.err
}
func (fb *fakeBuilderV2) Build(ctx context.Context, options *types.BuildOptions) error {
	return fb.build
}

func TestDoBuild(t *testing.T) {
	ctx := context.TODO()
	manifest := &model.Manifest{
		Build: build.ManifestBuild{
			"svc1": {
				Image: "image1",
			},
			"svc2": {
				Image: "image2",
			},
		},
		Test: map[string]*model.Test{
			"svc1": {
				Image: "image1",
			},
			"svc2": {
				Image: "image2",
			},
			"svc3": {
				Image: "$OKTETO_BUILD_NOIMAGE_IMAGE",
			},
		},
	}

	ioCtrl := io.NewIOController()

	type input struct {
		builder *fakeBuilderV2
		svcs    []string
	}
	type expected struct {
		wasBuilt bool
		err      error
	}

	tt := []struct {
		name     string
		input    input
		expected expected
	}{
		{
			name: "image on okteto variable is not in build section",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: buildv2.ErrOktetBuildSyntaxImageIsNotInBuildSection,
						svc: "",
					},
				},
				svcs: []string{"svc3"},
			},
			expected: expected{
				err:      fmt.Errorf("the image '$OKTETO_BUILD_NOIMAGE_IMAGE' is not in the build section of the manifest"),
				wasBuilt: false,
			},
		},
		{
			name: "another error on get svc to build from regex",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: assert.AnError,
						svc: "",
					},
				},
				svcs: []string{"svc3"},
			},
			expected: expected{
				err:      nil,
				wasBuilt: false,
			},
		},
		{
			name: "error checking services to build during deploy",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: nil,
						svc: "svc1",
					},
					svcToBuildDuringDeploy: fakeSvcToBuildDuringDeploy{
						err:  assert.AnError,
						svcs: []string{},
					},
				},
				svcs: []string{"svc1"},
			},
			expected: expected{
				err:      assert.AnError,
				wasBuilt: false,
			},
		},
		{
			name: "error on build",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: nil,
						svc: "svc1",
					},
					svcToBuildDuringDeploy: fakeSvcToBuildDuringDeploy{
						err:  nil,
						svcs: []string{"svc1"},
					},
					build: assert.AnError,
				},
				svcs: []string{"svc1"},
			},
			expected: expected{
				err:      assert.AnError,
				wasBuilt: true,
			},
		},
		{
			name: "buildSuccess",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: nil,
						svc: "svc1",
					},
					svcToBuildDuringDeploy: fakeSvcToBuildDuringDeploy{
						err:  nil,
						svcs: []string{"svc1"},
					},
					build: nil,
				},
				svcs: []string{"svc1"},
			},
			expected: expected{
				err:      nil,
				wasBuilt: true,
			},
		},
	}

	for _, tt := range tt {
		t.Run(tt.name, func(t *testing.T) {
			wasBuilt, err := doBuild(ctx, manifest, tt.input.svcs, tt.input.builder, ioCtrl)
			if tt.expected.err != nil {
				assert.ErrorContains(t, err, tt.expected.err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected.wasBuilt, wasBuilt)
		})
	}
}
