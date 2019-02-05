package cmd

import (
	"context"
	"os"
	"runtime"
	"sync"

	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/syncthing"
	getter "github.com/hashicorp/go-getter"
)

var (
	downloadPath = map[string]string{
		"linux":   "https://s3-us-west-1.amazonaws.com/okteto-cli/syncthing-1.0.0/linux/syncthing",
		"darwin":  "https://s3-us-west-1.amazonaws.com/okteto-cli/syncthing-1.0.0/darwin/syncthing",
		"windows": "https://s3-us-west-1.amazonaws.com/okteto-cli/syncthing-1.0.0/windows/syncthing.exe",
	}
)

func downloadSyncthing(ctx context.Context) error {
	opts := []getter.ClientOption{getter.WithProgress(defaultProgressBar)}

	client := &getter.Client{
		Ctx:     ctx,
		Src:     downloadPath[runtime.GOOS],
		Dst:     syncthing.GetInstallPath(),
		Mode:    getter.ClientModeFile,
		Options: opts,
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	errChan := make(chan error, 2)
	doneCh := make(chan struct{})
	go func() {
		defer wg.Done()
		if err := client.Get(); err != nil {
			log.Infof("failed to download syncthing from %s: %s", client.Src, err)
			os.Remove(client.Dst)
			errChan <- err
			return
		}

		if err := os.Chmod(client.Dst, 0700); err != nil {
			errChan <- err
			return
		}

		doneCh <- struct{}{}
		return
	}()

	select {
	case <-doneCh:
		wg.Wait()
		return nil
	case err := <-errChan:
		wg.Wait()
		return err
	}
}
