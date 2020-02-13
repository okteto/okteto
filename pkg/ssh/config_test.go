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

package ssh

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	sshConfigExample = `VisualHostKey yes

# dev
Host dev
  HostName 127.0.0.1
  User ubuntu
  Port 22
Host *.google.com *.yahoo.com
  User root
`
)

func TestWriteToNewFile(t *testing.T) {
	config, err := parse(strings.NewReader(sshConfigExample))
	if err != nil {
		t.Fatal(err)
	}

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(d)
	path := filepath.Join(d, "config")

	if err := config.writeToFilepath(path); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = parse(f)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseAndWriteTo(t *testing.T) {

	config, err := parse(strings.NewReader(sshConfigExample))
	if err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}

	if err := config.writeTo(buf); err != nil {
		t.Fatal(err)
	}

	if buf.String() != sshConfigExample {

		fmt.Println("==== expected")
		fmt.Println(sshConfigExample)
		fmt.Println("==== actual")
		fmt.Println(buf.String())
		fmt.Println("====")

		t.Errorf("input output mismatch")
	}

}
