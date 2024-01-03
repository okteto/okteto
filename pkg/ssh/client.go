// Copyright 2023 The Okteto Authors
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
	"fmt"
	"os"
	"sync"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"golang.org/x/crypto/ssh"
)

var clientConfig *ssh.ClientConfig
var timeout time.Duration
var tOnce sync.Once

func getPrivateKey() (ssh.Signer, error) {
	_, private := getKeyPaths()
	buf, err := os.ReadFile(private)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}

	key, err := ssh.ParsePrivateKey(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return key, nil
}

func getOktetoSSHTimeout() time.Duration {
	tOnce.Do(func() {
		timeout = 10 * time.Second
		t, ok := os.LookupEnv(model.OktetoSSHTimeoutEnvVar)
		if !ok {
			return
		}

		parsed, err := time.ParseDuration(t)
		if err != nil {
			oktetoLog.Infof("'%s' is not a valid duration, ignoring", t)
			return
		}

		oktetoLog.Infof("OKTETO_SSH_TIMEOUT applied: '%s'", parsed.String())
		timeout = parsed
	})

	return timeout
}

func getSSHClientConfig() (*ssh.ClientConfig, error) {
	if clientConfig != nil {
		return clientConfig, nil
	}

	keys, err := getPrivateKey()
	if err != nil {
		return nil, err
	}

	clientConfig = &ssh.ClientConfig{
		// skipcq GSC-G106
		// Ignoring this issue since the remote server doesn't have a set identity, and it's already secured by the
		// port-forward tunnel to the kubernetes cluster.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(keys),
		},
		Timeout: getOktetoSSHTimeout(),
	}

	return clientConfig, nil
}
