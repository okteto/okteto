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

package syncthing

import (
	"runtime"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
)

func TestInstall(t *testing.T) {
	if runtime.GOOS != "windows" {
		// we only test this on windows because linux is already covered by CI/CD in the integration test
		t.Skip("this test is only required for windows")
	}

	if err := Install(nil); err != nil {
		t.Fatal(err)
	}

	v := getInstalledVersion()
	if v == nil {
		t.Fatal("failed to get version")
	}

	if v.Compare(minimumVersion) != 0 {
		t.Fatalf("got %s, expected %s", v.String(), minimumVersion.String())
	}
}

func Test_getBinaryPathInDownload(t *testing.T) {
	p := getBinaryPathInDownload("dir", downloadURLs[runtime.GOOS])

	if !strings.Contains(p, minimumVersion.String()) {
		t.Errorf("got %s, expected to include %s", p, minimumVersion.String())
	}

	if !strings.HasSuffix(p, getBinaryName()) {
		t.Errorf("got %s, expected to finish with %s", p, getBinaryName())
	}
}

func Test_parseVersionFromOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  []byte
		want    *semver.Version
		wantErr bool
	}{
		{
			name:   "basic",
			output: []byte(`syncthing v1.13.0 "Fermium Flea" (go1.15 darwin-amd64) teamcity@build.syncthing.net 2021-01-11 14:15:21 UTC`),
			want:   semver.MustParse("1.13.0"),
		},
		{
			name:   "rc",
			output: []byte(`syncthing v1.13.0-rc.1 "Fermium Flea" (go1.16.1 darwin-arm64) teamcity@build.syncthing.net 2021-01-11 14:15:21 UTC`),
			want:   semver.MustParse("1.13.0-rc.1"),
		},
		{
			name:    "empty",
			output:  []byte(``),
			wantErr: true,
		},
		{
			name:    "no version",
			output:  []byte(`this string doesn't have a version`),
			wantErr: true,
		},
		{
			name:    "incomplete",
			output:  []byte(`syncthing v1.13 "Fermium Flea"`),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVersionFromOutput(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseVersionFromOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.want == nil {
				return
			}

			if !got.Equal(tt.want) {
				t.Errorf("parseVersionFromOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}
