package kubetoken

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestByteStore(t *testing.T) {

	// Test file does not exist

	t.Run("file does not exist", func(t *testing.T) {
		calledStat := false
		calledCreate := false
		calledReadFile := false

		f := FileByteStore{
			FileName: "does-not-exist",
			osStat: func(name string) (os.FileInfo, error) {
				require.Equal(t, "does-not-exist", name)
				calledStat = true
				return nil, os.ErrNotExist
			},
			osCreate: func(name string) (*os.File, error) {
				require.Equal(t, "does-not-exist", name)
				calledCreate = true
				return nil, nil
			},
			osReadFile: func(filename string) ([]byte, error) {
				require.Equal(t, "does-not-exist", filename)
				calledReadFile = true
				return nil, nil
			},
		}

		contents, err := f.Get()
		require.NoError(t, err)
		require.True(t, calledStat)
		require.True(t, calledCreate)
		require.True(t, calledReadFile)
		require.Empty(t, contents)
	})

	// Test file does not exist and cannot be created

	// Test error while writing file

	// Test error while stat file

}
