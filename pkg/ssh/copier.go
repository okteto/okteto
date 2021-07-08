// Copyright 2021 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
