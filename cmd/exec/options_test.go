package exec

import (
	"testing"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestNewOptions(t *testing.T) {
	testCases := []struct {
		name          string
		argsIn        []string
		argsLenAtDash int
		expected      *Options
	}{
		{
			name:          "Empty args",
			argsIn:        []string{},
			argsLenAtDash: 0,
			expected: &Options{
				command:     []string{},
				devSelector: utils.NewOktetoSelector("Select which development container to exec:", "Development container"),
			},
		},
		{
			name:          "Args with dev name",
			argsIn:        []string{"dev1"},
			argsLenAtDash: -1,
			expected: &Options{
				devName:           "dev1",
				firstArgIsDevName: true,
				command:           []string{},
				devSelector:       utils.NewOktetoSelector("Select which development container to exec:", "Development container"),
			},
		},
		{
			name:          "Args with command",
			argsIn:        []string{"echo", "test"},
			argsLenAtDash: 0,
			expected: &Options{
				command:     []string{"echo", "test"},
				devSelector: utils.NewOktetoSelector("Select which development container to exec:", "Development container"),
			},
		},
		{
			name:          "Args with dev name and command",
			argsIn:        []string{"dev1", "echo", "test"},
			argsLenAtDash: 1,
			expected: &Options{
				devName:           "dev1",
				firstArgIsDevName: true,
				command:           []string{"echo", "test"},
				devSelector:       utils.NewOktetoSelector("Select which development container to exec:", "Development container"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := NewOptions(tc.argsIn, tc.argsLenAtDash)
			assert.Equal(t, tc.expected, opts)
		})
	}
}

type fakeDevSelector struct {
	devName string
	err     error
}

func (f *fakeDevSelector) AskForOptionsOkteto([]utils.SelectorItem, int) (string, error) {
	return f.devName, f.err
}
func TestSetDevFromManifest(t *testing.T) {
	ioControl := io.NewIOController()
	// Define test cases using a slice of structs
	testCases := []struct {
		name          string
		options       *Options
		devs          model.ManifestDevs
		expectedDev   string
		expectedError error
	}{
		{
			name: "Dev name already set",
			options: &Options{
				devName: "dev1",
			},
			devs:          model.ManifestDevs{},
			expectedDev:   "dev1",
			expectedError: nil,
		},
		{
			name: "Select dev from manifest",
			options: &Options{
				devName: "",
				devSelector: &fakeDevSelector{
					devName: "dev1",
					err:     nil,
				},
			},
			devs: model.ManifestDevs{
				"dev1": &model.Dev{},
				"dev2": &model.Dev{},
			},
			expectedDev:   "dev1",
			expectedError: nil,
		},
		{
			name: "Failed to select dev",
			options: &Options{
				devName: "",
				devSelector: &fakeDevSelector{
					devName: "",
					err:     assert.AnError,
				},
			},
			devs:          model.ManifestDevs{},
			expectedDev:   "",
			expectedError: assert.AnError,
		},
	}

	// Loop through test cases and run assertions
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.options.setDevFromManifest(tc.devs, ioControl)
			assert.Equal(t, tc.expectedDev, tc.options.devName)
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func TestValidate(t *testing.T) {
	// Define test cases using a slice of structs
	testCases := []struct {
		name     string
		options  *Options
		devs     model.ManifestDevs
		expected error
	}{
		{
			name: "Missing dev name",
			options: &Options{
				command: []string{"echo", "test"},
			},
			devs:     model.ManifestDevs{},
			expected: errDevNameRequired,
		},
		{
			name: "Missing command",
			options: &Options{
				devName: "dev1",
			},
			devs:     model.ManifestDevs{},
			expected: errCommandRequired,
		},
		{
			name: "Invalid dev name (not defined)",
			options: &Options{
				devName: "dev2",
				command: []string{"echo", "test"},
			},
			devs:     model.ManifestDevs{},
			expected: &errDevNotInManifest{devName: "dev2"},
		},
		{
			name: "Valid options",
			options: &Options{
				devName: "dev1",
				command: []string{"echo", "test"},
			},
			devs: model.ManifestDevs{
				"dev1": &model.Dev{},
			},
			expected: nil,
		},
	}

	// Loop through test cases and run assertions
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.options.Validate(tc.devs)
			if tc.expected != nil {
				assert.ErrorContains(t, err, tc.expected.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
