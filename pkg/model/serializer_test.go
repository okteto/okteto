package model

import (
	"os"
	"reflect"
	"testing"

	yaml "gopkg.in/yaml.v2"
)

func TestEnvVarMashalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected EnvVar
	}{
		{
			"key-value",
			[]byte(`env=production`),
			EnvVar{Name: "env", Value: "production"},
		},
		{
			"key-value-complex",
			[]byte(`env='production=11231231asa#$˜GADAFA'`),
			EnvVar{Name: "env", Value: "'production=11231231asa#$˜GADAFA'"},
		},
		{
			"key-value-with-env-var",
			[]byte(`env=$DEV_ENV`),
			EnvVar{Name: "env", Value: "test_environment"},
		},
		{
			"key-value-with-env-var-in-string",
			[]byte(`env=my_env;$DEV_ENV;prod`),
			EnvVar{Name: "env", Value: "my_env;test_environment;prod"},
		},
		{
			"simple-key",
			[]byte(`noenv`),
			EnvVar{Name: "noenv", Value: ""},
		},
		{
			"key-with-no-value",
			[]byte(`noenv=`),
			EnvVar{Name: "noenv", Value: ""},
		},
		{
			"key-with-env-var-not-defined",
			[]byte(`noenv=$UNDEFINED`),
			EnvVar{Name: "noenv", Value: ""},
		},
		{
			"just-env-var",
			[]byte(`$DEV_ENV`),
			EnvVar{Name: "test_environment", Value: ""},
		},
		{
			"just-env-var-undefined",
			[]byte(`$UNDEFINED`),
			EnvVar{Name: "", Value: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var result EnvVar
			if err := os.Setenv("DEV_ENV", "test_environment"); err != nil {
				t.Fatal(err)
			}

			if err := yaml.Unmarshal(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", result, tt.expected)
			}

			_, err := yaml.Marshal(&result)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVolumeMashalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected Volume
	}{
		{
			"global",
			[]byte("/path"),
			Volume{SubPath: "", MountPath: "/path"},
		},
		{
			"relative",
			[]byte("sub:/path"),
			Volume{SubPath: "sub", MountPath: "/path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v Volume
			if err := yaml.Unmarshal(tt.data, &v); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(v, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %s, Expected %s", v, tt.expected)
			}

			_, err := yaml.Marshal(&v)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
