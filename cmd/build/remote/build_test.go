package remote

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/internal/test"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

type fakeRegistry struct {
	registry map[string]fakeImage
}

// fakeImage represents the data from an image
type fakeImage struct {
	Registry string
	Repo     string
	Tag      string
	ImageRef string
	Args     []string
}

func newFakeRegistry() fakeRegistry {
	return fakeRegistry{
		registry: map[string]fakeImage{},
	}
}

func (fr fakeRegistry) GetImageTagWithDigest(imageTag string) (string, error) {
	if _, ok := fr.registry[imageTag]; !ok {
		return "", oktetoErrors.ErrNotFound
	}
	return imageTag, nil
}

func (fr fakeRegistry) AddImageByOpts(opts *types.BuildOptions) error {
	fr.registry[opts.Tag] = fakeImage{Args: opts.BuildArgs}
	return nil
}

func (fr fakeRegistry) IsOktetoRegistry(image string) bool { return false }
func (fr fakeRegistry) HasGlobalPushAccess() (bool, error) { return false, nil }
func (fr fakeRegistry) IsGlobalRegistry(image string) bool { return false }

func (fr fakeRegistry) GetRegistryAndRepo(image string) (string, string) { return "", "" }
func (fr fakeRegistry) GetRepoNameAndTag(repo string) (string, string)   { return "", "" }

func TestBuildWithErrorFromDockerfile(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry, fmt.Errorf("failed to build error"))
	bc := &OktetoBuilder{
		Builder:  builder,
		Registry: registry,
	}
	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	tag := "okteto.dev/test"
	options := &types.BuildOptions{
		CommandArgs: []string{dir},
		Tag:         tag,
	}
	err = bc.Build(ctx, options)

	// error from the build
	assert.Error(t, err)
	// the image is not at the fake registry
	image, err := bc.Registry.GetImageTagWithDigest(tag)
	assert.ErrorIs(t, err, oktetoErrors.ErrNotFound)
	assert.Empty(t, image)
}

func TestBuildWithNoErrorFromDockerfile(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	bc := &OktetoBuilder{
		Builder:  builder,
		Registry: registry,
	}
	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	tag := "okteto.dev/test"
	options := &types.BuildOptions{
		CommandArgs: []string{dir},
		Tag:         tag,
	}
	err = bc.Build(ctx, options)
	// no error from the build
	assert.NoError(t, err)
	// the image is at the fake registry
	image, err := bc.Registry.GetImageTagWithDigest(tag)
	assert.NoError(t, err)
	assert.NotEmpty(t, image)
}

func TestBuildWithNoErrorFromDockerfileAndNoTag(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	bc := &OktetoBuilder{
		Builder:  builder,
		Registry: registry,
	}
	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	options := &types.BuildOptions{
		CommandArgs: []string{dir},
	}
	err = bc.Build(ctx, options)
	// no error from the build
	assert.NoError(t, err)
	// the image is not at the fake registry
	image, err := bc.Registry.GetImageTagWithDigest("")
	assert.ErrorIs(t, err, oktetoErrors.ErrNotFound)
	assert.Empty(t, image)
}

func createDockerfile(t *testing.T) (string, error) {
	dir := t.TempDir()
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	err := os.WriteFile(dockerfilePath, []byte("Hello"), 0600)
	if err != nil {
		return "", err
	}
	return dir, nil
}
