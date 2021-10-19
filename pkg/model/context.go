package model

import (
	"os"

	"github.com/okteto/okteto/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type ContextResource struct {
	Context   string
	Namespace string
}

// GetContextResource returns a ContextResource object from a given file
func GetContextResource(filePath string) (*ContextResource, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	ctxResource := &ContextResource{}
	if err := yaml.Unmarshal(bytes, ctxResource); err != nil {
		return nil, err
	}

	ctxResource.Context = os.ExpandEnv(ctxResource.Context)
	ctxResource.Namespace = os.ExpandEnv(ctxResource.Namespace)

	return ctxResource, nil
}

func (c *ContextResource) UpdateContext(okCtx string) error {
	if c.Context != "" {
		if okCtx != "" && okCtx != c.Context {
			return errors.ErrContextNotMatching
		}
		return nil
	}

	c.Context = okCtx
	return nil
}

func (c *ContextResource) UpdateNamespace(okNs string) error {
	if c.Namespace != "" {
		if okNs != "" && c.Namespace != okNs {
			return errors.ErrNamespaceNotMatching
		}
		return nil
	}

	c.Namespace = okNs
	return nil
}
