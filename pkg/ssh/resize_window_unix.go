//go:build !windows
// +build !windows

package ssh

import (
	"os"
	"os/signal"
	"syscall"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func resizeWindow(session *ssh.Session) {
	resize := make(chan os.Signal, 1)
	signal.Notify(resize, syscall.SIGWINCH)
	go func() {
		for range resize {
			width, height, err := term.GetSize(int(os.Stdout.Fd()))
			oktetoLog.Infof("terminal width %d height %d", width, height)
			if err != nil {
				oktetoLog.Infof("request for terminal size failed: %s", err)
			}
			session.WindowChange(height, width)
		}
	}()
}
