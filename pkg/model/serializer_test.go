package model

import (
	"os"
	"reflect"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v2"
)

func TestReverseMashalling(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		expected  Reverse
		expectErr bool
	}{
		{
			name:     "basic",
			data:     "8080:9090",
			expected: Reverse{Local: 9090, Remote: 8080},
		},
		{
			name:     "equal",
			data:     "8080:8080",
			expected: Reverse{Local: 8080, Remote: 8080},
		},
		{
			name:      "missing-part",
			data:      "8080",
			expectErr: true,
		},
		{
			name:      "non-integer",
			data:      "8080:foo",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Reverse
			if err := yaml.Unmarshal([]byte(tt.data), &result); err != nil {
				if tt.expectErr {
					return
				}

				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual '%+v', Expected '%+v'", result, tt.expected)
			}

			out, err := yaml.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}

			outStr := string(out)
			outStr = strings.TrimSuffix(outStr, "\n")

			if !reflect.DeepEqual(outStr, tt.data) {
				t.Errorf("didn't unmarshal correctly. Actual '%+v', Expected '%+v'", outStr, tt.data)
			}
		})
	}
}

func TestForwardMashalling(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		expected  Forward
		expectErr bool
	}{
		{
			name:     "basic",
			data:     "8080:9090",
			expected: Forward{Local: 8080, Remote: 9090},
		},
		{
			name:     "equal",
			data:     "8080:8080",
			expected: Forward{Local: 8080, Remote: 8080},
		},
		{
			name:      "missing-part",
			data:      "8080",
			expectErr: true,
		},
		{
			name:      "non-integer",
			data:      "8080:foo",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Forward
			if err := yaml.Unmarshal([]byte(tt.data), &result); err != nil {
				if tt.expectErr {
					return
				}

				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual '%+v', Expected '%+v'", result, tt.expected)
			}

			out, err := yaml.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}

			outStr := string(out)
			outStr = strings.TrimSuffix(outStr, "\n")

			if !reflect.DeepEqual(outStr, tt.data) {
				t.Errorf("didn't unmarshal correctly. Actual '%+v', Expected '%+v'", outStr, tt.data)
			}
		})
	}
}
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

func TestEnvVar_UnmarshalYAML(t *testing.T) {
	type fields struct {
		Name  string
		Value string
	}
	type args struct {
		unmarshal func(interface{}) error
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EnvVar{
				Name:  tt.fields.Name,
				Value: tt.fields.Value,
			}
			if err := e.UnmarshalYAML(tt.args.unmarshal); (err != nil) != tt.wantErr {
				t.Errorf("EnvVar.UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestForward_UnmarshalYAML(t *testing.T) {
	type fields struct {
		Local  int
		Remote int
	}
	type args struct {
		unmarshal func(interface{}) error
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Forward{
				Local:  tt.fields.Local,
				Remote: tt.fields.Remote,
			}
			if err := f.UnmarshalYAML(tt.args.unmarshal); (err != nil) != tt.wantErr {
				t.Errorf("Forward.UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReverse_UnmarshalYAML(t *testing.T) {
	type fields struct {
		Remote int
		Local  int
	}
	type args struct {
		unmarshal func(interface{}) error
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Reverse{
				Remote: tt.fields.Remote,
				Local:  tt.fields.Local,
			}
			if err := f.UnmarshalYAML(tt.args.unmarshal); (err != nil) != tt.wantErr {
				t.Errorf("Reverse.UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResourceList_UnmarshalYAML(t *testing.T) {
	type args struct {
		unmarshal func(interface{}) error
	}
	tests := []struct {
		name    string
		r       *ResourceList
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.r.UnmarshalYAML(tt.args.unmarshal); (err != nil) != tt.wantErr {
				t.Errorf("ResourceList.UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVolume_UnmarshalYAML(t *testing.T) {
	type fields struct {
		SubPath   string
		MountPath string
	}
	type args struct {
		unmarshal func(interface{}) error
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Volume{
				SubPath:   tt.fields.SubPath,
				MountPath: tt.fields.MountPath,
			}
			if err := v.UnmarshalYAML(tt.args.unmarshal); (err != nil) != tt.wantErr {
				t.Errorf("Volume.UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
