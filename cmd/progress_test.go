package cmd

import "testing"

func Test_renderProgressBar(t *testing.T) {
	var tests = []struct {
		expected string
		progress float64
		name     string
	}{
		{
			expected: "[__________________________________________________]   0%",
			progress: 0.0,
			name:     "no-progress",
		},
		{
			expected: "[-------------->___________________________________]  30%",
			progress: 30.0,
			name:     "progress-30",
		},
		{
			expected: "[--------------------------------------------------] 100%",
			progress: 100.0,
			name:     "progress-100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := renderProgressBar(tt.progress, 0.5)
			if tt.expected != actual {
				t.Errorf("\nexpected:\n%s\ngot:\n%s", tt.expected, actual)
			}
		})
	}

}
