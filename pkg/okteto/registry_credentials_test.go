package okteto

import (
	"context"
	"fmt"
	"testing"

	dockertypes "github.com/docker/cli/cli/config/types"
	"github.com/stretchr/testify/require"
)

type getFn func(ctx context.Context, host string) (dockertypes.AuthConfig, error)

func TestExternalRegistryCredentialsOK(t *testing.T) {
	defaultV1 := "https://index.docker.io/v1/"
	defaultV2 := "https://index.docker.io/v2/"

	spyOnAndExpect := func(expected string) getFn {
		return func(ctx context.Context, host string) (dockertypes.AuthConfig, error) {
			require.Equal(t, expected, host)
			return dockertypes.AuthConfig{
				Username: "user",
				Password: "pass",
			}, nil
		}
	}

	tt := []struct {
		input    string
		getter   getFn
		expected [2]string
	}{
		// default hub v1 registry
		{
			input:  "index.docker.io",
			getter: spyOnAndExpect(defaultV1),
		},
		{
			input:  "index.docker.io/v1",
			getter: spyOnAndExpect(defaultV1),
		},
		{
			input:  "index.docker.io/v1/",
			getter: spyOnAndExpect(defaultV1),
		},
		{
			input:  "https://index.docker.io",
			getter: spyOnAndExpect(defaultV1),
		},
		{
			input:  "http://index.docker.io",
			getter: spyOnAndExpect(defaultV1),
		},
		{
			input:  "https://index.docker.io/v1",
			getter: spyOnAndExpect(defaultV1),
		},
		{
			input:  "http://index.docker.io/v1",
			getter: spyOnAndExpect(defaultV1),
		},
		{
			input:  "https://index.docker.io/v1/",
			getter: spyOnAndExpect(defaultV1),
		},
		{
			input:  "http://index.docker.io/v1/",
			getter: spyOnAndExpect(defaultV1),
		},
		// v2 hub registry
		{
			input:  "index.docker.io/v2",
			getter: spyOnAndExpect(defaultV2),
		},
		{
			input:  "index.docker.io/v2/",
			getter: spyOnAndExpect(defaultV2),
		},
		{
			input:  "https://index.docker.io/v2",
			getter: spyOnAndExpect(defaultV2),
		},
		{
			input:  "http://index.docker.io/v2",
			getter: spyOnAndExpect(defaultV2),
		},
		{
			input:  "https://index.docker.io/v2/",
			getter: spyOnAndExpect(defaultV2),
		},
		{
			input:  "http://index.docker.io/v2/",
			getter: spyOnAndExpect(defaultV2),
		},
		{
			input:  "http://index.docker.io/v2/",
			getter: spyOnAndExpect(defaultV2),
		},
		// external registries
		{
			input:  "https://gcr.io",
			getter: spyOnAndExpect("gcr.io"),
		},
		{
			input:  "http://gcr.io",
			getter: spyOnAndExpect("gcr.io"),
		},
		{
			input:  "https://gcr.io/qwerty",
			getter: spyOnAndExpect("gcr.io"),
		},
		{
			input:  "https://gcr.io/qwerty/nested/path/12345",
			getter: spyOnAndExpect("gcr.io"),
		},
		{
			input:  "some-extranous-host/with-strange-path",
			getter: spyOnAndExpect("some-extranous-host"),
		},
		{
			input:  "https://gcr.io?with=query-string#and-fragment",
			getter: spyOnAndExpect("gcr.io"),
		},
	}

	ctx := context.Background()
	for i, tc := range tt {
		name := fmt.Sprintf("check%v", i)
		t.Run(name, func(t *testing.T) {
			r := externalRegistryCredentialsReader{
				isOkteto: true,
				getter:   tc.getter,
			}
			user, pass, err := r.read(ctx, tc.input)
			require.NoError(t, err)
			require.Equal(t, "user", user)
			require.Equal(t, "pass", pass)
		})

	}

}

func TestExternalRegistryCredentialsOKWithCache(t *testing.T) {
	var calls int

	r := externalRegistryCredentialsReader{
		isOkteto: true,
		cache:    &registryCache{},
		getter: func(ctx context.Context, host string) (dockertypes.AuthConfig, error) {
			calls++
			require.Equal(t, "host.com", host)
			return dockertypes.AuthConfig{
				Username: "user",
				Password: "pass",
			}, nil
		},
	}

	for i := 0; i < 20; i++ {
		user, pass, err := r.read(context.Background(), "host.com")
		require.NoError(t, err)
		require.Equal(t, "user", user)
		require.Equal(t, "pass", pass)
	}

	// We made 20 calls to r.read and getter should've only been executed once
	require.Equal(t, 1, calls)
}

func TestRegistryCache(t *testing.T) {
	rc := registryCache{}
	user, pass, ok := rc.Get("host.com")
	require.Equal(t, "", user)
	require.Equal(t, "", pass)
	require.False(t, ok)

	rc.Set("thebiglebowski.com", "thedude", "elduderino")

	user, pass, ok = rc.Get("thebiglebowski.com")
	require.Equal(t, "thedude", user)
	require.Equal(t, "elduderino", pass)
	require.True(t, ok)
}
