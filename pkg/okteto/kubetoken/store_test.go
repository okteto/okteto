// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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
			createFile: func(name string) (*os.File, error) {
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
			createFile: func(name string) (*os.File, error) {
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
