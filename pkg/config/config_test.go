package config

import (
	"os"
	"path"
	"testing"
)

func TestGetCNDHome(t *testing.T) {
	tests := []struct {
		name   string
		want   string
		config *Config
	}{
		{
			"default", path.Join(os.Getenv("HOME"), ".cnd"), &Config{},
		},
		{
			"custom-path", "/tmp/foo/.cnd", &Config{CNDHomePath: "/tmp/foo"},
		},
		{
			"custom-path-and-name", "/tmp/foo/.bar", &Config{CNDHomePath: "/tmp/foo", CNDFolderName: ".bar"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetConfig(tt.config)
			if got := GetCNDHome(); got != tt.want {
				t.Errorf("GetCNDHome() = %v, want %v", got, tt.want)
			}
		})
	}
}
