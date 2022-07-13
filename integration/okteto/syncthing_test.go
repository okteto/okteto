//go:build integration
// +build integration

// Copyright 2022 The Okteto Authors
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

package okteto

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/okteto/okteto/pkg/syncthing"
)

func TestDownloadSyncthing(t *testing.T) {
	var tests = []struct {
		os   string
		arch string
	}{
		{os: "windows", arch: "amd64"},
		{os: "darwin", arch: "amd64"},
		{os: "darwin", arch: "arm64"},
		{os: "linux", arch: "amd64"},
		{os: "linux", arch: "arm64"},
		{os: "linux", arch: "arm"},
	}

	ctx := context.Background()
	m := syncthing.GetMinimumVersion()
	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("%s-%s", tt.os, tt.arch), func(t *testing.T) {
			t.Parallel()
			u, err := syncthing.GetDownloadURL(tt.os, tt.arch, m.String())
			req, err := http.NewRequest("GET", u, nil)
			if err != nil {
				t.Fatal(err.Error())
			}

			req = req.WithContext(ctx)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to download syncthing: %s", err)
			}

			if res.StatusCode != 200 {
				t.Fatalf("Failed to download syncthing. Got status: %d", res.StatusCode)
			}
		})
	}
}
