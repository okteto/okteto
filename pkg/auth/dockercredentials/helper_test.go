package dockercredentials

import (
	"errors"
	"os"
	"testing"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/stretchr/testify/require"
)

var errNotFound = errors.New("fake creds not found")

type fakeGetter struct {
	creds map[string][2]string
}

func (fg fakeGetter) GetRegistryCredentials(host string) (string, string, error) {
	if fg.creds == nil {
		fg.creds = make(map[string][2]string)
	}

	if creds, ok := fg.creds[host]; ok {
		return creds[0], creds[1], nil
	}

	return "", "", errNotFound

}

func TestAddDelete(t *testing.T) {
	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	h := OktetoClusterHelper{
		dirname: dir,
		getter:  fakeGetter{},
	}
	creds := &credentials.Credentials{
		ServerURL: "registry.com",
		Username:  "lebowski",
		Secret:    "thedude",
	}

	err = h.Add(creds)
	require.NoError(t, err)
	user, pass, err := h.Get("registry.com")
	require.NoError(t, err)
	require.Equal(t, "lebowski", user)
	require.Equal(t, "thedude", pass)

	err = h.Delete("registry.com")
	require.NoError(t, err)

	_, _, err = h.Get("registry.com")
	require.ErrorIs(t, err, errNotFound)
}
