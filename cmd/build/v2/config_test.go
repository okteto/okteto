package v2

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

type fakeConfigRegistry struct {
	access bool
	err    error
}

func (fcr fakeConfigRegistry) HasGlobalPushAccess() (bool, error) { return fcr.access, fcr.err }

type fakeConfigRepo struct {
	sha     string
	isClean bool
	err     error
}

func (fcr fakeConfigRepo) GetSHA() (string, error)             { return fcr.sha, fcr.err }
func (fcr fakeConfigRepo) IsCleanContext(string) (bool, error) { return fcr.isClean, fcr.err }

func TestGetConfig(t *testing.T) {
	type input struct {
		reg  fakeConfigRegistry
		repo fakeConfigRepo
	}
	tt := []struct {
		name     string
		input    input
		expected oktetoBuilderConfig
	}{
		{
			name: "global access",
			input: input{
				reg: fakeConfigRegistry{
					access: true,
					err:    nil,
				},
				repo: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: true,
				fs:              afero.NewOsFs(),
				isOkteto:        true,
			},
		},
		{
			name: "no global access",
			input: input{
				reg: fakeConfigRegistry{
					access: false,
					err:    nil,
				},
				repo: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: false,
				fs:              afero.NewOsFs(),
				isOkteto:        true,
			},
		},
		{
			name: "error on global access",
			input: input{
				reg: fakeConfigRegistry{
					access: false,
					err:    assert.AnError,
				},
				repo: fakeConfigRepo{
					isClean: true,
					err:     assert.AnError,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: false,
				fs:              afero.NewOsFs(),
				isOkteto:        true,
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := getConfig(tc.input.reg)
			assert.Equal(t, tc.expected, cfg)
		})
	}
}
