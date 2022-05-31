//go:build windows
// +build windows

package ssh

import (
	"os"
	"time"

	dockerterm "github.com/moby/term"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"golang.org/x/crypto/ssh"
)

func resizeWindow(session *ssh.Session) {
	go func() {
		prevWSize, err := dockerterm.GetWinsize(os.Stdout.Fd())
		if err != nil {
			oktetoLog.Infof("request for terminal size failed: %s", err)
		}
		for {
			time.Sleep(time.Millisecond * 250)
			ws, err := dockerterm.GetWinsize(os.Stdout.Fd())
			if err != nil {
				oktetoLog.Infof("request for terminal size failed: %s", err)
			}
			if prevWSize.Height != ws.Height || prevWSize.Width != ws.Width {
				if err := dockerterm.SetWinsize(os.Stdout.Fd(), ws); err != nil {
					oktetoLog.Infof("request for terminal resize failed: %s", err)
				}
			}
			prevWSize = ws
		}
	}()
}
