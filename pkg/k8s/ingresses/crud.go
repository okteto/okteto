package ingresses

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Client struct {
	c    kubernetes.Interface
	isV1 bool
}

type Ingress struct {
	V1      *networkingv1.Ingress
	V1Beta1 *networkingv1beta1.Ingress
}

func GetClient(ctx context.Context, c *kubernetes.Clientset) (*Client, error) {
	rList, err := c.ServerResourcesForGroupVersion("networking.k8s.io/v1")
	if err != nil {
		return nil, err
	}
	for _, apiResource := range rList.APIResources {
		if apiResource.Kind == "Ingress" {
			return &Client{
				c:    c,
				isV1: true,
			}, nil
		}
	}

	return &Client{
		c:    c,
		isV1: false,
	}, nil
}

//Get results the ingress
func (iClient *Client) Get(ctx context.Context, name, namespace string) (metav1.Object, error) {
	if iClient.isV1 {
		i, err := iClient.c.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return i.GetObjectMeta(), nil
	}

	i, err := iClient.c.NetworkingV1beta1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return i.GetObjectMeta(), nil
}

func (iClient *Client) GetHosts(ctx context.Context, name, namespace string) ([]string, error) {
	hosts := make([]string, 0)

	if iClient.isV1 {
		i, err := iClient.c.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		for _, rule := range i.Spec.Rules {
			hosts = append(hosts, rule.Host)
		}
		return hosts, nil
	}

	i, err := iClient.c.NetworkingV1beta1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	for _, rule := range i.Spec.Rules {
		hosts = append(hosts, rule.Host)
	}
	return hosts, nil
}

func (iClient *Client) Create(ctx context.Context, i *Ingress) error {
	if iClient.isV1 {
		_, err := iClient.c.NetworkingV1().Ingresses(i.V1.Namespace).Create(ctx, i.V1, metav1.CreateOptions{})
		return err
	}
	_, err := iClient.c.NetworkingV1beta1().Ingresses(i.V1Beta1.Namespace).Create(ctx, i.V1Beta1, metav1.CreateOptions{})
	return err
}

//Update updates a statefulset
func (iClient *Client) Update(ctx context.Context, i *Ingress) error {
	if iClient.isV1 {
		_, err := iClient.c.NetworkingV1().Ingresses(i.V1.Namespace).Update(ctx, i.V1, metav1.UpdateOptions{})
		return err
	}
	_, err := iClient.c.NetworkingV1beta1().Ingresses(i.V1Beta1.Namespace).Update(ctx, i.V1Beta1, metav1.UpdateOptions{})
	return err
}

//List returns the list of deployments
func (iClient *Client) List(ctx context.Context, namespace, labels string) ([]metav1.Object, error) {
	result := []metav1.Object{}
	if iClient.isV1 {
		iList, err := iClient.c.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels})
		if err != nil {
			return nil, err
		}
		for i := range iList.Items {
			result = append(result, iList.Items[i].GetObjectMeta())
		}
		return result, nil
	}

	iList, err := iClient.c.NetworkingV1beta1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels})
	if err != nil {
		return nil, err
	}
	for i := range iList.Items {
		result = append(result, iList.Items[i].GetObjectMeta())
	}
	return result, nil
}

//Destroy destroys a k8s deployment
func (iClient *Client) Destroy(ctx context.Context, name, namespace string) error {
	log.Infof("deleting ingress '%s'", name)
	if iClient.isV1 {
		err := iClient.c.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("error deleting kubernetes ingress: %s", err)
		}
		return nil
	}

	err := iClient.c.NetworkingV1beta1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error deleting kubernetes ingress: %s", err)
	}
	return nil
}
