package cp

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mholt/archiver"
	"github.com/okteto/cnd/pkg/k8/exec"
	"github.com/okteto/cnd/pkg/model"
	uuid "github.com/satori/go.uuid"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var cpCommand = []string{"tar", "-xzf", "-", "--strip-components=1", "-C", "/src"}

// Copy copies a local folder to the remote volume
func Copy(c *kubernetes.Clientset, config *rest.Config, namespace string, pod *apiv1.Pod, folder string) error {
	dir := os.TempDir()
	tarfile := filepath.Join(dir, fmt.Sprintf("tarball-%s.tgz", uuid.NewV4().String()))
	if err := archiver.Archive([]string{folder}, tarfile); err != nil {
		return err
	}
	defer os.Remove(tarfile)

	file, err := os.Open(tarfile)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(file)
	if err := exec.Exec(c, config, pod, model.CNDInitSyncContainerName, false, reader, os.Stdout, os.Stderr, cpCommand); err != nil {
		return err
	}
	return nil
}
