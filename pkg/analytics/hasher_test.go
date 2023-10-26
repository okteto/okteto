package analytics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_hashString(t *testing.T) {
	input := "test-string"
	require.Equal(t, hashString(input), "ffe65f1d98fafedea3514adc956c8ada5980c6c5d2552fd61f48401aefd5c00e")
}
