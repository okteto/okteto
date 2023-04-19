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

package model

import (
	"fmt"
	"net"
	"strconv"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// GetAvailablePort returns a random port that's available
func GetAvailablePort(iface string) (int, error) {
	hostAndPort := net.JoinHostPort(iface, strconv.Itoa(0))
	address, err := net.ResolveTCPAddr("tcp", hostAndPort)
	if err != nil {
		return 0, fmt.Errorf("error resolving address for %q: %w", hostAndPort, err)
	}

	listener, err := net.ListenTCP("tcp", address)
	if err != nil {
		return 0, fmt.Errorf("error listening on port %q: %w", address, err)
	}

	defer func() {
		if err := listener.Close(); err != nil {
			oktetoLog.Debugf("Error closing listener: %w", err)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, nil

}

// IsPortAvailable returns true if the port is already taken
func IsPortAvailable(iface string, port int) bool {
	address := net.JoinHostPort(iface, strconv.Itoa(port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		oktetoLog.Infof("port %s is taken: %s", address, err)
		return false
	}

	defer func() {
		if err := listener.Close(); err != nil {
			oktetoLog.Debugf("Error closing file %s: %s", listener, err)
		}
	}()
	return true
}
