package cmd

import (
	"testing"

	"github.com/Masterminds/semver"
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
			if got := shouldNotify(tt.args.latest, tt.args.current); got != tt.want {
				t.Errorf("shouldNotify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetVersion(t *testing.T) {
	v, err := getVersion()
	if err != nil {
		t.Fatal(err)
	}

	_, err = semver.NewVersion(v)
	if err != nil {
		t.Fatal(err)
	}
}
