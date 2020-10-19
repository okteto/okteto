// Copyright 2020 The Okteto Authors
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
	"testing"
)

func TestGetAvailablePort(t *testing.T) {
	p, err := GetAvailablePort()
	if err != nil {
		t.Fatal(err)
	}

	if p == 0 {
		t.Fatal("got an empty port")
	}
}

func TestIsPortAvailable(t *testing.T) {
	p, err := GetAvailablePort()
	if err != nil {
		t.Fatal(err)
	}

	if !IsPortAvailable(p) {
		t.Fatalf("port %d wasn't available", p)
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
	if err != nil {
		t.Fatal(err)
	}

	defer l.Close()

	if IsPortAvailable(p) {
		t.Fatalf("port %d was available", p)
	}
}
