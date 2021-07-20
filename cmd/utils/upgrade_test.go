// Copyright 2021 The Okteto Authors
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

package utils

import (
	"testing"

	"github.com/Masterminds/semver/v3"
)

func Test_shouldNotify(t *testing.T) {
	one, _ := semver.NewVersion("1.0.0")
	oneZeroOne, _ := semver.NewVersion("1.0.1")
	oneOneZero, _ := semver.NewVersion("1.1.0")
	two, _ := semver.NewVersion("2.0.0")

	type args struct {
		latest  *semver.Version
		current *semver.Version
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "equal", args: args{latest: oneOneZero, current: oneOneZero}, want: false},
		{name: "patch", args: args{latest: oneZeroOne, current: one}, want: false},
		{name: "minor", args: args{latest: oneOneZero, current: oneZeroOne}, want: true},
		{name: "major", args: args{latest: two, current: oneOneZero}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldNotify(tt.args.latest, tt.args.current); got != tt.want {
				t.Errorf("shouldNotify() = %v, want %v", got, tt.want)
			}
		})
	}
}
