package externalresource

import (
	"context"
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/constants"
	oerrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/externalresource/k8s"
	"github.com/okteto/okteto/pkg/format"
	olog "github.com/okteto/okteto/pkg/log"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// K8sControl represents the controller that performs the actions with k8s
type K8sControl struct {
	ClientProvider func(clientcmdapi.Config) (k8s.ExternalResourceV1Interface, error)
	Cfg            *clientcmdapi.Config
}

func (c *K8sControl) Deploy(ctx context.Context, name, ns string, externalInfo *ExternalResource) error {
	k8sclient, err := c.ClientProvider(*c.Cfg)
	if err != nil {
		return fmt.Errorf("error creating external CRD client: %s", err.Error())
	}

	externalResourceCRD := translate(name, externalInfo)

	old, err := k8sclient.ExternalResources(ns).Get(ctx, externalResourceCRD.Name, metav1.GetOptions{})
	if err != nil && !oerrors.IsNotFound(err) {
		return fmt.Errorf("error getting external resource CRD '%s': %w", externalResourceCRD.Name, err)
	}

	if old.Name == "" {
		olog.Infof("creating external resource CRD '%s'", externalResourceCRD.Name)
		_, err = k8sclient.ExternalResources(ns).Create(ctx, externalResourceCRD)
		if err != nil && !k8sErrors.IsAlreadyExists(err) {
			return fmt.Errorf("error creating external resource CRD '%s': %w", externalResourceCRD.Name, err)
		}
		olog.Infof("created external resource CRD '%s'", externalResourceCRD.Name)
	} else {
		olog.Infof("updating external resource CRD '%s'", externalResourceCRD.Name)
		old.TypeMeta = externalResourceCRD.TypeMeta
		old.Annotations = externalResourceCRD.Annotations
		old.Labels = externalResourceCRD.Labels
		old.Spec = externalResourceCRD.Spec
		_, err = k8sclient.ExternalResources(ns).Update(ctx, old)
		if err != nil {
			if !k8sErrors.IsConflict(err) {
				return fmt.Errorf("error updating external resource CRD '%s': %w", externalResourceCRD.Name, err)
			}
		}
		olog.Infof("updated external resource CRD '%s'.", externalResourceCRD.Name)
	}

	return nil
}

func translate(name string, externalResource *ExternalResource) *k8s.External {
	var externalEndpointsSpec []k8s.Endpoint
	for _, endpoint := range externalResource.Endpoints {
		externalEndpointsSpec = append(externalEndpointsSpec, k8s.Endpoint(endpoint))
	}

	return &k8s.External{
		TypeMeta: metav1.TypeMeta{
			Kind:       k8s.ExternalResourceKind,
			APIVersion: fmt.Sprintf("%s/%s", k8s.GroupName, k8s.GroupVersion),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: format.ResourceK8sMetaString(name),
			Annotations: map[string]string{
				constants.LastUpdatedAnnotation: time.Now().UTC().Format(constants.TimeFormat),
			},
		},
		Spec: k8s.ExternalResourceSpec{
			Name: format.ResourceK8sMetaString(name),
			Notes: &k8s.Notes{
				Path:     externalResource.Notes.Path,
				Markdown: externalResource.Notes.Markdown,
			},
			Endpoints: externalEndpointsSpec,
		},
	}
}
