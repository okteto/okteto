package analytics

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/okteto"
)

func Test_generatedMachineID(t *testing.T) {
	m := generateMachineID()
	if len(m) == 0 || m == "na" {
		t.Fatalf("bad machineID: %s", m)
	}

	for i := 0; i < 10; i++ {
		nM := generateMachineID()
		if nM != m {
			t.Fatalf("different machineID.\ngot: %s\nexpected: %s", nM, m)
		}
	}
}

func Test_getTrackID(t *testing.T) {
	var tests = []struct {
		name      string
		machineID string
		userID    string
	}{
		{name: "new", machineID: ""},
		{name: "existing-machine-id", machineID: "machine-123"},
		{name: "existing-user-id", machineID: "machine-123", userID: "user-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			os.Setenv("HOME", dir)
			os.Setenv("OKTETO_TOKEN", "")

			if len(tt.machineID) > 0 {
				if err := okteto.SaveMachineID(tt.machineID); err != nil {
					t.Fatal(err)
				}
			}

			if len(tt.userID) > 0 {
				if err := okteto.SaveID(tt.userID); err != nil {
					t.Fatal(err)
				}
			}

			trackID := getTrackID()
			if len(trackID) == 0 || trackID == "na" {
				t.Fatalf("failed to get trackID: %s", trackID)
			}

			expected := ""
			if len(tt.userID) > 0 {
				expected = tt.userID
			} else if len(tt.machineID) > 0 {
				expected = tt.machineID
			} else {
				expected = getMachineID()
			}

			if trackID != expected {
				t.Fatalf("different trackID.\ngot: %s\nexpected: %s", trackID, expected)
			}
		})
	}
}
