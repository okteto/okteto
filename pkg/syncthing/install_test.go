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
)

func Test_downloadSyncthing(t *testing.T) {
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
