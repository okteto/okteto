package okteto

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

func Test_save(t *testing.T) {
	var tests = []struct {
		name  string
		token *Token
	}{
		{
			name: "basic", token: &Token{ID: "1234", Token: "ABCDEFG", URL: "http://example.com"},
		},
		{
			name: "with-machineid", token: &Token{ID: "1234", Token: "ABCDEFG", URL: "http://example.com", MachineID: "machine-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentToken = nil
			dir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			os.Setenv("HOME", dir)
			os.Setenv("OKTETO_TOKEN", "")

			if err := save(tt.token); err != nil {
				t.Fatal(err)
			}

			token, err := getToken()
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(token, tt.token) {
				t.Fatalf("\ngot:\n%+v\nexpected:\n%+v", token, tt.token)
			}
		})
	}
}

func TestSaveMachineID(t *testing.T) {
	var tests = []struct {
		name      string
		existing  *Token
		machineID string
		expected  *Token
	}{
		{
			name:      "no-existing",
			machineID: "machine-1",
			existing:  nil,
			expected:  &Token{ID: "", URL: "", Token: "", MachineID: "machine-1"},
		},
		{
			name:      "existing",
			machineID: "machine-1",
			existing:  &Token{ID: "123", URL: "http://example.com", Token: "ABCDEFG"},
			expected:  &Token{ID: "123", URL: "http://example.com", Token: "ABCDEFG", MachineID: "machine-1"},
		},
		{
			name:      "replacing",
			machineID: "machine-3",
			existing:  &Token{ID: "123", URL: "http://example.com", Token: "ABCDEFG", MachineID: "machine-2"},
			expected:  &Token{ID: "123", URL: "http://example.com", Token: "ABCDEFG", MachineID: "machine-3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentToken = nil
			dir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			os.Setenv("HOME", dir)
			os.Setenv("OKTETO_TOKEN", "")

			if tt.existing != nil {
				if err := save(tt.existing); err != nil {
					t.Fatal(err)
				}
			}

			if err := SaveMachineID(tt.machineID); err != nil {
				t.Fatal(err)
			}

			token, err := getToken()
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(token, tt.expected) {
				t.Fatalf("\ngot:\n%+v\nexpected:\n%+v", token, tt.expected)
			}
		})
	}
}
