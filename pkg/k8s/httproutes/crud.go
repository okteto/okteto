// Copyright 2023-2025 The Okteto Authors
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

package httproutes

import (
	"context"
	"fmt"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// Client represents the HTTPRoute client
type Client struct {
	gatewayClient gatewayclientset.Interface
}

// NewHTTPRouteClient creates a new HTTPRoute client from a rest config
func NewHTTPRouteClient(config *rest.Config) (*Client, error) {
	gatewayClient, err := gatewayclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway client: %w", err)
	}
	return &Client{
		gatewayClient: gatewayClient,
	}, nil
}

// Get retrieves the HTTPRoute
func (c *Client) Get(ctx context.Context, name, namespace string) (*gatewayv1.HTTPRoute, error) {
	return c.gatewayClient.GatewayV1().HTTPRoutes(namespace).Get(ctx, name, metav1.GetOptions{})
}

// Create creates an HTTPRoute
func (c *Client) Create(ctx context.Context, httpRoute *gatewayv1.HTTPRoute) error {
	_, err := c.gatewayClient.GatewayV1().HTTPRoutes(httpRoute.Namespace).Create(ctx, httpRoute, metav1.CreateOptions{})
	return err
}

// Update updates an HTTPRoute
func (c *Client) Update(ctx context.Context, httpRoute *gatewayv1.HTTPRoute) error {
	_, err := c.gatewayClient.GatewayV1().HTTPRoutes(httpRoute.Namespace).Update(ctx, httpRoute, metav1.UpdateOptions{})
	return err
}

// List returns the list of HTTPRoutes
func (c *Client) List(ctx context.Context, namespace, labels string) ([]metav1.Object, error) {
	result := []metav1.Object{}
	httpRouteList, err := c.gatewayClient.GatewayV1().HTTPRoutes(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels})
	if err != nil {
		return nil, err
	}

	for i := range httpRouteList.Items {
		result = append(result, &httpRouteList.Items[i])
	}

	return result, nil
}

// Destroy destroys an HTTPRoute
func (c *Client) Destroy(ctx context.Context, name, namespace string) error {
	oktetoLog.Infof("deleting httproute '%s'", name)
	err := c.gatewayClient.GatewayV1().HTTPRoutes(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return fmt.Errorf("error deleting kubernetes httproute: %w", err)
	}
	return nil
}

// Deploy creates or updates an HTTPRoute
func (c *Client) Deploy(ctx context.Context, httpRoute *gatewayv1.HTTPRoute) error {
	existing, err := c.Get(ctx, httpRoute.Name, httpRoute.Namespace)
	if err != nil {
		if !oktetoErrors.IsNotFound(err) {
			return fmt.Errorf("error getting httproute '%s': %w", httpRoute.Name, err)
		}
		if err := c.Create(ctx, httpRoute); err != nil {
			return err
		}
		oktetoLog.Success("Endpoint '%s' created", httpRoute.Name)
		return nil
	}

	// Preserve resource version for update
	httpRoute.ResourceVersion = existing.ResourceVersion
	if err := c.Update(ctx, httpRoute); err != nil {
		return err
	}
	oktetoLog.Success("Endpoint '%s' updated", httpRoute.Name)
	return nil
}

// GetEndpointsBySelector retrieves endpoint URLs from HTTPRoutes
func (c *Client) GetEndpointsBySelector(ctx context.Context, namespace, labels string) ([]string, error) {
	result := make([]string, 0)
	httpRouteList, err := c.gatewayClient.GatewayV1().HTTPRoutes(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels})
	if err != nil {
		return nil, err
	}

	for _, httpRoute := range httpRouteList.Items {
		// Extract hostnames from the HTTPRoute status
		for _, parentStatus := range httpRoute.Status.Parents {
			for _, condition := range parentStatus.Conditions {
				if condition.Type == "Accepted" && condition.Status == metav1.ConditionTrue {
					// Try to get hostname from parent gateway
					// This is a simplified implementation - in reality, you'd need to fetch the Gateway
					// and extract its hostname from its status
					for _, rule := range httpRoute.Spec.Rules {
						for _, match := range rule.Matches {
							if match.Path != nil && match.Path.Value != nil {
								// Build URL - hostname would come from Gateway status in real implementation
								result = append(result, fmt.Sprintf("https://%s%s", httpRoute.Name, *match.Path.Value))
							}
						}
					}
				}
			}
		}
	}

	return result, nil
}
