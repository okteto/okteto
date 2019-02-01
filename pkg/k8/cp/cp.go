package cp

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudnativedevelopment/cnd/pkg/k8/exec"
	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/model"
	"github.com/mholt/archiver"
	uuid "github.com/satori/go.uuid"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var tarCommand = []string{"tar", "-xzf", "-", "--strip-components=1", "-C", "/src"}
var touchCommand = []string{"touch", "/initialized"}

// Copy copies a local folder to the remote volume
func Copy(ctx context.Context, c *kubernetes.Clientset, config *rest.Config, namespace string, pod *apiv1.Pod, dev *model.Dev) error {
	dir := os.TempDir()
	tarfile := filepath.Join(dir, fmt.Sprintf("tarball-%s.tgz", uuid.NewV4().String()))
	if err := archiver.Archive([]string{dev.Mount.Source}, tarfile); err != nil {
		log.Errorf("failed to tar source folder: %s", err.Error())
		return fmt.Errorf("Failed to tar source folder")
	}
	defer os.Remove(tarfile)

	file, err := os.Open(tarfile)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(file)
	log.Info("sending tarball...")
	if err := exec.Exec(ctx, c, config, pod, dev.GetCNDInitSyncContainer(), false, reader, os.Stdout, os.Stderr, tarCommand); err != nil {
		log.Errorf("failed to sent tarball: %s", err.Error())
		return fmt.Errorf("Failed to send tarball")
	}
	log.Info("Tarball sent")
	if err := exec.Exec(ctx, c, config, pod, dev.GetCNDInitSyncContainer(), false, os.Stdin, os.Stdout, os.Stderr, touchCommand); err != nil {
		log.Errorf("failed to sent initialized flag: %s", err.Error())
		return fmt.Errorf("Failed to send initialized flag")
	}
	return nil
}
