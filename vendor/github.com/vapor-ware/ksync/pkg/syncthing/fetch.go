package syncthing

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jpillora/overseer/fetcher"
	log "github.com/sirupsen/logrus"
)

// syncthing renames the OS for mac to macos instead of darwin.
func matchRelease(filename string) bool {
	os := runtime.GOOS
	if os == "darwin" {
		os = "macos"
	}

	return strings.Contains(filename, os) &&
		strings.Contains(filename, runtime.GOARCH)
}

func saveBinary(reader io.Reader, path string) error { //nolint interfacer
	dir := filepath.Dir(path)
	if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
		if mkdirErr := os.Mkdir(dir, 0700); mkdirErr != nil {
			return mkdirErr
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0500)
	if err != nil {
		return err
	}
	defer f.Close() // nolint: errcheck

	if _, err := io.Copy(f, reader); err != nil {
		return err
	}

	log.Debug("wrote syncthing binary")

	return nil
}

// Fetch pulls down the latest syncthing binary to the provided path.
func Fetch(path string) error {
	f := &fetcher.Github{
		User:  "syncthing",
		Repo:  "syncthing",
		Asset: matchRelease,
	}

	if err := f.Init(); err != nil {
		return err
	}

	log.Debug("fetching new syncthing binary")

	archiveReader, err := f.Fetch()
	if err != nil {
		return err
	}

	var binaryReader io.Reader
	switch runtime.GOOS {
	case "windows":
		log.Debug("found windows binary")
		binaryReader, err = UnpackWindows(archiveReader)
	// We should do some other platform detection here for completeness
	default:
		log.Debug("found binary")
		binaryReader, err = UnpackNix(archiveReader)
	}
	if err != nil {
		return err
	}

	return saveBinary(binaryReader, path)
}

// UnpackNix upacks the tarball and returns a reader containing the binary
func UnpackNix(reader io.Reader) (io.Reader, error) {
	log.Debug("decompressing")

	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()

		if err != nil {
			return nil, err
		}

		// There are config files that are named the same thing as the binary. As
		// they're in etc directories, ignore those too.
		if strings.HasSuffix(header.Name, "/syncthing") &&
			!strings.Contains(header.Name, "/etc/") {
			return tarReader, nil
		}
	}
}

// UnpackWindows unpacks the zip archive and returns a reader containing the binary
func UnpackWindows(reader io.Reader) (io.Reader, error) {
	log.Debug("decompressing")

	// `encoding/tar` and `encoding/zip` are implemented just differently enough
	// to force us into doing all this stupid shit. See what you've reduced me to?
	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	readerAt := bytes.NewReader(b)

	zipReader, err := zip.NewReader(readerAt, getSize(readerAt))
	if err != nil {
		return nil, err
	}

	for _, f := range zipReader.File {
		if strings.HasSuffix(f.Name, "syncthing.exe") && !strings.Contains(f.Name, "sig") {
			file, err := f.Open()
			return file, err
		}
	}

	return nil, fmt.Errorf("no syncthing binary found")
}

// getSize returns the size of an arbitrary io.Reader
func getSize(stream io.Reader) int64 {
	buf := new(bytes.Buffer)
	buf.ReadFrom(stream) // nolint: errcheck, gas, gosec
	return int64(buf.Len())
}
