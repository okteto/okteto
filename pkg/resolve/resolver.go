package resolve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/schollz/progressbar/v3"
)

type OktetoContext struct {
	Name     string `json:"name"`
	IsOkteto bool   `json:"isOkteto"`
}

type OktetoContextFile struct {
	CurrentContext string                    `json:"current-context"`
	Contexts       map[string]*OktetoContext `json:"contexts"`
}

type ClusterInfo struct {
	ClusterVersion string `json:"clusterVersion"`
}

type Resolver struct {
	PullOptions PullOptions
}

func (r *Resolver) CurrentContext() (*OktetoContext, error) {
	f := oktetoContextFilename()
	c := OktetoContextFile{}

	ctxFile, err := os.Open(f)
	if err != nil {
		return nil, err
	}
	defer ctxFile.Close()
	if err := json.NewDecoder(ctxFile).Decode(&c); err != nil {
		return nil, err
	}

	if len(c.Contexts) < 1 {
		return nil, nil
	}

	return c.Contexts[c.CurrentContext], nil
}

// Resolve resolves the okteto binary that will be executed as a sub-process
// based on the current context
func (r *Resolver) Resolve(ctx context.Context) (string, error) {
	okCtx, err := r.CurrentContext()
	if err != nil {
		return "", err
	}
	var version string

	switch {

	// the OKTETO_FORCE_VERSION envvar allows to override the version bin
	case os.Getenv("OKTETO_FORCE_VERSION") != "":
		version = os.Getenv("OKTETO_FORCE_VERSION")

	// if there is no context use the latest known version
	case okCtx == nil:
		version = LatestVersionAlias

	// if it's not an okteto cluster use the latest known version
	case !okCtx.IsOkteto:
		version = LatestVersionAlias

	default:
		version, err = r.fetchClusterVersion(ctx, okCtx.Name)
		if err != nil {
			return "", fmt.Errorf("failed to fetch cluster version: %v", err)
		}
	}

	if version == LatestVersionAlias {
		version, err = FindLatest(ctx, FindLatestOptions{
			PullHost:   r.PullOptions.PullHost,
			Channel:    r.PullOptions.Channel,
			HTTPClient: r.PullOptions.HTTPClient,
		})
		if err != nil {
			return "", err
		}
	}
	binFile := path.Join(versionedBinDir(), fmt.Sprintf("okteto-%s", version))

	if _, err := os.Stat(binFile); errors.Is(err, os.ErrNotExist) {
		if err := r.pullWithProgress(ctx, version); err != nil {
			return "", err
		}
	}
	return binFile, nil
}

func (r *Resolver) pullWithProgress(ctx context.Context, version string) error {
	progress, err := Pull(ctx, version, r.PullOptions)
	if err != nil {
		return err
	}
	pb := progressbar.DefaultBytes(100)
	for p := range progress {
		if pb.GetMax64() != p.Size {
			pb.ChangeMax64(p.Size)
		}
		pb.Add64(p.Completed)
		if p.Done {
			pb.Finish()
			pb.Clear()
			if p.Error != nil {
				return p.Error
			}
		}
	}
	return nil
}

func (r *Resolver) fetchClusterVersion(ctx context.Context, host string) (version string, err error) {
	var info *ClusterInfo
	info, err = r.fetchClusterInfo(ctx, host)
	if err == nil {
		version = info.ClusterVersion
	}
	return
}
func (r *Resolver) fetchClusterInfo(ctx context.Context, host string) (*ClusterInfo, error) {
	c := http.Client{
		Timeout:   time.Second * 10,
		Transport: defaultTransport,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/clusterinfo", host), nil)
	if err != nil {
		return nil, err
	}

	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		// for backwards compatiblity, if the endpoint doesn't exist return "latest"
		return &ClusterInfo{
			ClusterVersion: LatestVersionAlias,
		}, nil
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected error getting %s/clusterinfo: %s", host, res.Status)
	}

	// add an upper bound to the response body of 10mb
	reader := io.LimitReader(res.Body, 10*1024*1024)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var info *ClusterInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	if info.ClusterVersion == "" || info.ClusterVersion == "0.0.0" {
		info.ClusterVersion = LatestVersionAlias
	}

	return info, nil
}
