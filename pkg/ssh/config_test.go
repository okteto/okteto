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
