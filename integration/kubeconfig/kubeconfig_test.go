package kubeconfig

import (
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/stretchr/testify/require"
)

// Test_KubeconfigHasExec kubeconfig command should use exec instead of token for the user auth
func Test_KubeconfigHasExec(t *testing.T) {
	t.Parallel()

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	home := t.TempDir()

	err = commands.RunOktetoKubeconfig(oktetoPath, home)
	require.NoError(t, err)

	cfg := kubeconfig.Get([]string{filepath.Join(home, ".kube", "config")})
	require.Len(t, cfg.AuthInfos, 1)

	for _, v := range cfg.AuthInfos {
		require.NotNil(t, v)
		require.Empty(t, v.Token)
		require.NotNil(t, v.Exec)
	}
}
