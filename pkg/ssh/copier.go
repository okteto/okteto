package ssh

import (
	"io"
	"time"

	"github.com/okteto/okteto/pkg/log"
)

// Copier copies from local to remote terminalhandles the lifecycle of all the forwards
type Copier struct {
	Local  io.Reader
	Remote io.WriteCloser
}

var copier *Copier

func Copy(local io.Reader, remote io.WriteCloser) {
	if copier == nil {
		copier = &Copier{Local: local, Remote: remote}
		go copier.Copy()
		return
	}
	copier.Remote = remote
}

func (c *Copier) Copy() {
	buf := make([]byte, 32*1024)
	t := time.NewTicker(100 * time.Millisecond)
	for {
		nr, er := c.Local.Read(buf)
		write := 0
		for write < nr {
			nw, ew := c.Remote.Write(buf[write:nr])
			write += nw
			if ew != nil {
				log.Infof("write to remote terminal error: %s", ew.Error())
				<-t.C
				continue
			}
		}
		if er != nil {
			log.Infof("read from local to remote terminal error: %s", er.Error())
			return
		}
	}
}
