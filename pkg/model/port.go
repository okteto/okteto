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
	"math/rand"
	"net"
	"strconv"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// GetAvailablePort returns a random port that's available
func GetAvailablePort(iface string) (int, error) {
	address, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(iface, strconv.Itoa(0)))
	if err != nil {
		return 0, err
	}

	listener, err := net.ListenTCP("tcp", address)
	if err != nil {
		return 0, err
	}

	defer func() {
		if err := listener.Close(); err != nil {
			oktetoLog.Debugf("Error closing listener: %s", err)
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

// GetAvailablePortInRange returns the first available port in the specified range,
// starting from a random offset to reduce collisions in parallel builds
func GetAvailablePortInRange(iface string, minPort, maxPort int) (int, error) {
	rangeSize := maxPort - minPort + 1
	startOffset := rand.Intn(rangeSize)

	// Try ports starting from random offset, wrapping around
	for i := 0; i < rangeSize; i++ {
		port := minPort + (startOffset+i)%rangeSize
		if IsPortAvailable(iface, port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", minPort, maxPort)
}
