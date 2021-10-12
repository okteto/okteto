package model

import (
	"os"

	"github.com/okteto/okteto/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type ContextResource struct {
	Context   string `yaml:"context,omitempty"`
	Namespace string `yaml:"namespace,omitempty"`
	Token     string
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

func (c *ContextResource) UpdateNamespace(namespace string) error {
	if c.Namespace != "" {
		if namespace != "" && c.Namespace != namespace {
			return errors.ErrNamespaceNotMatching
		}
		return nil
	}

	if namespace == "" && os.Getenv("OKTETO_NAMESPACE") != "" {
		namespace = os.Getenv("OKTETO_NAMESPACE")
	}

	c.Namespace = namespace
	return nil
}

func (c *ContextResource) UpdateContext(okCtx string) error {
	if c.Context != "" {
		if okCtx != "" && okCtx != c.Context {
			return errors.ErrContextNotMatching
		}
		return nil
	}

	if okCtx == "" && os.Getenv("OKTETO_URL") != "" {
		okCtx = os.Getenv("OKTETO_URL")
	}

	if okCtx == "" && os.Getenv("OKTETO_CONTEXT") != "" {
		okCtx = os.Getenv("OKTETO_CONTEXT")
	}

	c.Context = okCtx
	return nil
}
