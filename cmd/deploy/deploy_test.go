package deploy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

var fakeManifest *Manifest = &Manifest{
	Deploy: []string{
		"printenv",
		"ls -la",
		"cat /tmp/test.txt",
	},
}

type fakeProxy struct {
	errOnStart    error
	errOnShutdown error
	port          int
	token         string
	started       bool
	shutdown      bool
}

type fakeExecutor struct {
	err      error
	executed []string
}

type fakeKubeConfig struct {
	errOnRead   error
	errOnModify error
}

func (fc *fakeKubeConfig) Read() (*rest.Config, error) {
	if fc.errOnRead != nil {
		return nil, fc.errOnRead
	}

	return &rest.Config{}, nil
}

func (fc *fakeKubeConfig) Modify(ctx context.Context, port int, sessionToken string) error {
	return fc.errOnModify
}

func (fk *fakeProxy) Start(ctx context.Context, name string, clusterConfig *rest.Config) error {
	if fk.errOnStart != nil {
		return fk.errOnStart
	}

	fk.started = true
	return nil
}

func (fk *fakeProxy) Shutdown(ctx context.Context) error {
	if fk.errOnShutdown != nil {
		return fk.errOnShutdown
	}

	fk.shutdown = true
	return nil
}

func (fk *fakeProxy) GetPort() int {
	return fk.port
}

func (fk *fakeProxy) GetToken() string {
	return fk.token
}

func (fe *fakeExecutor) Execute(command string, env []string) error {
	fe.executed = append(fe.executed, command)
	if fe.err != nil {
		return fe.err
	}

	return nil
}

func TestDeployWithErrorReadingKubeConfig(t *testing.T) {
	e := &fakeExecutor{}
	p := &fakeProxy{}
	c := &deployCommand{
		proxy:    p,
		executor: e,
		kubeconfig: &fakeKubeConfig{
			errOnRead: assert.AnError,
		},
	}
	ctx := context.Background()
	cwd := "/tmp"
	name := "movies"
	filename := ""
	variables := []string{}

	err := c.runDeploy(ctx, cwd, name, filename, variables)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, e.executed, 0)
	// Proxy wasn't started
	assert.False(t, p.started)
}

func TestDeployWithErrorChangingKubeConfig(t *testing.T) {
	p := &fakeProxy{}
	e := &fakeExecutor{}
	c := &deployCommand{
		proxy:    p,
		executor: e,
		kubeconfig: &fakeKubeConfig{
			errOnModify: assert.AnError,
		},
	}
	ctx := context.Background()
	cwd := "/tmp"
	name := "movies"
	filename := ""
	variables := []string{}

	err := c.runDeploy(ctx, cwd, name, filename, variables)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, e.executed, 0)
	// Proxy wasn't started
	assert.False(t, p.started)
}

func TestDeployWithErrorReadingManifestFile(t *testing.T) {
	p := &fakeProxy{}
	e := &fakeExecutor{}
	c := &deployCommand{
		getManifest: getManifestWithError,
		proxy:       p,
		executor:    e,
		kubeconfig:  &fakeKubeConfig{},
	}
	ctx := context.Background()
	cwd := "/tmp"
	name := "movies"
	filename := ""
	variables := []string{}

	err := c.runDeploy(ctx, cwd, name, filename, variables)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, e.executed, 0)
	// Proxy wasn't started
	assert.False(t, p.started)
}

func TestDeployWithErrorStartingProxy(t *testing.T) {
	p := &fakeProxy{
		errOnStart: assert.AnError,
	}
	e := &fakeExecutor{}
	c := &deployCommand{
		getManifest: getFakeManifest,
		getSecrets:  getFakeSecrets,
		proxy:       p,
		executor:    e,
		kubeconfig:  &fakeKubeConfig{},
	}
	ctx := context.Background()
	cwd := "/tmp"
	name := "movies"
	filename := ""
	variables := []string{}

	err := c.runDeploy(ctx, cwd, name, filename, variables)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, e.executed, 0)
	// Proxy wasn't started
	assert.False(t, p.started)
}

func TestDeployWithErrorExecutingCommands(t *testing.T) {
	p := &fakeProxy{}
	e := &fakeExecutor{
		err: assert.AnError,
	}
	c := &deployCommand{
		getManifest: getFakeManifest,
		getSecrets:  getFakeSecrets,
		proxy:       p,
		executor:    e,
		kubeconfig:  &fakeKubeConfig{},
	}
	ctx := context.Background()
	cwd := "/tmp"
	name := "movies"
	filename := ""
	variables := []string{}

	err := c.runDeploy(ctx, cwd, name, filename, variables)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, e.executed, 1)
	// Check expected commands were executed
	assert.Equal(t, fakeManifest.Deploy[0], e.executed[0])
	// Proxy started
	assert.True(t, p.started)
	// Proxy shutdown
	assert.True(t, p.shutdown)
}

func TestDeployWithErrorShuttingdownProxy(t *testing.T) {
	p := &fakeProxy{
		errOnShutdown: assert.AnError,
	}
	e := &fakeExecutor{}
	c := &deployCommand{
		getManifest: getFakeManifest,
		getSecrets:  getFakeSecrets,
		proxy:       p,
		executor:    e,
		kubeconfig:  &fakeKubeConfig{},
	}
	ctx := context.Background()
	cwd := "/tmp"
	name := "movies"
	filename := ""
	variables := []string{}

	err := c.runDeploy(ctx, cwd, name, filename, variables)

	assert.Error(t, err)
	// No command was executed
	assert.Len(t, e.executed, 3)
	// Check expected commands were executed
	assert.Equal(t, fakeManifest.Deploy, e.executed)
	// Proxy started
	assert.True(t, p.started)
	// Proxy wasn't shutdown
	assert.False(t, p.shutdown)
}

func TestDeployWithoutErrors(t *testing.T) {
	p := &fakeProxy{}
	e := &fakeExecutor{}
	c := &deployCommand{
		getManifest: getFakeManifest,
		getSecrets:  getFakeSecrets,
		proxy:       p,
		executor:    e,
		kubeconfig:  &fakeKubeConfig{},
	}
	ctx := context.Background()
	cwd := "/tmp"
	name := "movies"
	filename := ""
	variables := []string{}

	err := c.runDeploy(ctx, cwd, name, filename, variables)

	assert.NoError(t, err)
	// No command was executed
	assert.Len(t, e.executed, 3)
	// Check expected commands were executed
	assert.Equal(t, fakeManifest.Deploy, e.executed)
	// Proxy started
	assert.True(t, p.started)
	// Proxy was shutdown
	assert.True(t, p.shutdown)
}

func getManifestWithError(_, _, _ string) (*Manifest, error) {
	return nil, assert.AnError
}

func getFakeManifest(_, _, _ string) (*Manifest, error) {
	return fakeManifest, nil
}

func getFakeSecrets(_ context.Context) ([]string, error) {
	return []string{}, nil
}
