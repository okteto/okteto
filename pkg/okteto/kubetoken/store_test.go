package kubetoken

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestByteStore(t *testing.T) {
	t.Run("file does not exist", func(t *testing.T) {
		calledStat := false
		calledCreate := false
		calledReadFile := false

		fileName := "does-not-exist"

		f := FileByteStore{
			FileName: fileName,
			osStat: func(name string) (os.FileInfo, error) {
				require.Equal(t, fileName, name)
				calledStat = true
				return nil, os.ErrNotExist
			},
			osCreate: func(name string) (*os.File, error) {
				require.Equal(t, fileName, name)
				calledCreate = true
				return nil, nil
			},
			osReadFile: func(name string) ([]byte, error) {
				require.Equal(t, fileName, name)
				calledReadFile = true
				return nil, nil
			},
		}

		contents, err := f.Get()
		require.Empty(t, contents)
		require.NoError(t, err)

		require.True(t, calledStat)
		require.True(t, calledCreate)
		require.True(t, calledReadFile)
	})

	t.Run("file does not exist, error while stating file", func(t *testing.T) {
		calledStat := false

		fileName := "does-not-exist"

		f := FileByteStore{
			FileName: fileName,
			osStat: func(name string) (os.FileInfo, error) {
				require.Equal(t, fileName, name)
				calledStat = true
				return nil, assert.AnError
			},
		}

		contents, err := f.Get()
		require.Empty(t, contents)
		require.Error(t, err)

		require.True(t, calledStat)
	})

	t.Run("file does not exist and cannot be created", func(t *testing.T) {
		calledStat := false
		calledCreate := false

		fileName := "does-not-exist"

		f := FileByteStore{
			FileName: fileName,
			osStat: func(name string) (os.FileInfo, error) {
				require.Equal(t, fileName, name)
				calledStat = true
				return nil, os.ErrNotExist
			},
			osCreate: func(name string) (*os.File, error) {
				require.Equal(t, fileName, name)
				calledCreate = true
				return nil, assert.AnError
			},
		}

		contents, err := f.Get()
		require.Empty(t, contents)
		require.Error(t, err)

		require.True(t, calledStat)
		require.True(t, calledCreate)
	})

	t.Run("file exists, error while reading file", func(t *testing.T) {
		calledStat := false
		calledReadFile := false

		fileName := "does-not-exist"

		f := FileByteStore{
			FileName: fileName,
			osStat: func(name string) (os.FileInfo, error) {
				require.Equal(t, fileName, name)
				calledStat = true
				return nil, nil
			},
			osReadFile: func(name string) ([]byte, error) {
				require.Equal(t, fileName, name)
				calledReadFile = true
				return nil, assert.AnError
			},
		}

		contents, err := f.Get()
		require.Empty(t, contents)
		require.Error(t, err)

		require.True(t, calledStat)
		require.True(t, calledReadFile)
	})

	t.Run("set", func(t *testing.T) {
		calledWriteFile := false

		fileName := "does-not-exist"

		errorReturned := assert.AnError

		f := FileByteStore{
			FileName: fileName,
			writeFile: func(name string, data []byte) error {
				require.Equal(t, fileName, name)
				calledWriteFile = true
				return errorReturned
			},
		}

		err := f.Set([]byte("hello"))
		require.Equal(t, errorReturned, err)

		require.True(t, calledWriteFile)
	})

}
