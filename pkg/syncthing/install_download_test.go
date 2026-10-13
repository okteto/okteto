// Copyright 2026 The Okteto Authors
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

package syncthing

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/stretchr/testify/require"
)

func tarGzArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     name,
		Mode:     0o4755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}))
	_, err := tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func zipArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
	hdr.SetMode(os.ModeSetuid | 0o755)
	w, err := zw.CreateHeader(hdr)
	require.NoError(t, err)
	_, err = w.Write(content)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestInstallDownloadsAndExtractsArchive(t *testing.T) {
	version := GetMinimumVersion().String()
	binary := []byte("fake syncthing binary")

	tests := []struct {
		archive func(*testing.T, string, []byte) []byte
		name    string
		ext     string
	}{
		{
			name:    "tar.gz",
			ext:     ".tar.gz",
			archive: tarGzArchive,
		},
		{
			name:    "zip",
			ext:     ".zip",
			archive: zipArchive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := tt.archive(t, fmt.Sprintf("syncthing-v%s/%s", version, getBinaryName()), binary)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(payload)
			}))
			t.Cleanup(server.Close)

			urlFormat := server.URL + "/syncthing-v%[1]s" + tt.ext
			original := downloadURLFormats
			t.Cleanup(func() { downloadURLFormats = original })
			downloadURLFormats = map[string]string{
				"linux":       urlFormat,
				"arm":         urlFormat,
				"arm64":       urlFormat,
				"darwinArm64": urlFormat,
				"darwin":      urlFormat,
				"windows":     urlFormat,
			}
			t.Setenv(constants.OktetoFolderEnvVar, t.TempDir())

			require.NoError(t, Install(nil))

			got, err := os.ReadFile(getInstallPath())
			require.NoError(t, err)
			require.Equal(t, binary, got)
		})
	}
}
