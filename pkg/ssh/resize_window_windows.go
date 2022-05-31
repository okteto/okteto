//go:build windows
// +build windows

package ssh

import (
	"os"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func resizeWindow(session *ssh.Session) {
	go func() {
		prevW, prevH, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil {
			oktetoLog.Infof("request for terminal size failed: %s", err)
		}
		for {
			time.Sleep(time.Millisecond * 250)
			w, h, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				oktetoLog.Infof("request for terminal size failed: %s", err)
			}
			if prevH != h || prevW != w {
				if err := session.WindowChange(h, w); err != nil {
					oktetoLog.Infof("request for terminal resize failed: %s", err)
				}
			}
			prevH = h
			prevW = w
		}
	}()
}
