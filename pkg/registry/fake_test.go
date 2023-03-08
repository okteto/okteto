package registry

import (
	"crypto/x509"

	containerv1 "github.com/google/go-containerregistry/pkg/v1"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/types"
)

// FakeOktetoRegistry emulates an okteto image registry
type FakeOktetoRegistry struct {
	Err      error
	Registry map[string]*FakeImage
}

// FakeImage represents the data from an image
type FakeImage struct {
	Registry string
	Repo     string
	Tag      string
	ImageRef string
	Args     []string
}

// NewFakeOktetoRegistry creates a new registry if not already created
func NewFakeOktetoRegistry(err error) *FakeOktetoRegistry {
	return &FakeOktetoRegistry{
		Err:      err,
		Registry: map[string]*FakeImage{},
	}
}

// AddImageByName adds an image to the registry
func (fb *FakeOktetoRegistry) AddImageByName(images ...string) error {
	for _, image := range images {
		fb.Registry[image] = &FakeImage{}
	}
	return nil
}

// AddImageByOpts adds an image to the registry
func (fb *FakeOktetoRegistry) AddImageByOpts(opts *types.BuildOptions) error {
	fb.Registry[opts.Tag] = &FakeImage{Args: opts.BuildArgs}
	return nil
}

// GetImageTagWithDigest returns the image tag digest
func (fb *FakeOktetoRegistry) GetImageTagWithDigest(imageTag string) (string, error) {
	if _, ok := fb.Registry[imageTag]; !ok {
		return "", oktetoErrors.ErrNotFound
	}
	return imageTag, nil
}

func (fb *FakeOktetoRegistry) GetImageMetadata(image string) (ImageMetadata, error) {
	return ImageMetadata{}, nil
}
func (fb *FakeOktetoRegistry) IsOktetoRegistry(image string) bool                  { return true }
func (fb *FakeOktetoRegistry) GetImageTag(image, service, namespace string) string { return "" }
func (fb *FakeOktetoRegistry) GetImageReference(image string) (OktetoImageReference, error) {
	return OktetoImageReference{}, nil
}

type FakeConfig struct {
	IsOktetoClusterCfg          bool
	GlobalNamespace             string
	Namespace                   string
	RegistryURL                 string
	UserID                      string
	Token                       string
	InsecureSkipTLSVerifyPolicy bool
	ContextCertificate          *x509.Certificate
}

func (fc FakeConfig) IsOktetoCluster() bool               { return fc.IsOktetoClusterCfg }
func (fc FakeConfig) GetGlobalNamespace() string          { return fc.GlobalNamespace }
func (fc FakeConfig) GetNamespace() string                { return fc.Namespace }
func (fc FakeConfig) GetRegistryURL() string              { return fc.RegistryURL }
func (fc FakeConfig) GetUserID() string                   { return fc.UserID }
func (fc FakeConfig) GetToken() string                    { return fc.Token }
func (fc FakeConfig) IsInsecureSkipTLSVerifyPolicy() bool { return fc.InsecureSkipTLSVerifyPolicy }
func (fc FakeConfig) GetContextCertificate() (*x509.Certificate, error) {
	return fc.ContextCertificate, nil
}

// FakeClient has everything needed to set up a test faking API calls
type FakeClient struct {
	GetImageDigest GetDigest
	GetConfig      GetConfig
}

// GetDigest has everything needed to mock a getDigest API call
type GetDigest struct {
	Result string
	Err    error
}

// GetConfig has everything needed to mock a getConfig API call
type GetConfig struct {
	Result *containerv1.ConfigFile
	Err    error
}

func (fc FakeClient) GetDigest(image string) (string, error) {
	return fc.GetImageDigest.Result, fc.GetImageDigest.Err
}

func (fc FakeClient) GetImageConfig(image string) (*containerv1.ConfigFile, error) {
	return fc.GetConfig.Result, fc.GetConfig.Err
}
