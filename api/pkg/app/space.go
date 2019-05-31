package app

import (
	"context"
	"fmt"

	"github.com/okteto/app/api/pkg/k8s/client"
	"github.com/okteto/app/api/pkg/k8s/limitranges"
	"github.com/okteto/app/api/pkg/k8s/namespaces"
	"github.com/okteto/app/api/pkg/k8s/networkpolicies"
	"github.com/okteto/app/api/pkg/k8s/podpolicies"
	"github.com/okteto/app/api/pkg/k8s/quotas"
	"github.com/okteto/app/api/pkg/k8s/rolebindings"
	"github.com/okteto/app/api/pkg/k8s/roles"
	"github.com/okteto/app/api/pkg/model"
	"github.com/opentracing/opentracing-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//CreateSpace configures a namespace for a given user
func CreateSpace(s *model.Space) error {
	c := client.Get()

	if err := namespaces.Create(s, c); err != nil {
		return err
	}

	if err := podpolicies.Create(s, c); err != nil {
		return err
	}

	if err := quotas.Create(s, c); err != nil {
		return err
	}

	if err := limitranges.Create(s, c); err != nil {
		return err
	}

	if err := roles.Create(s, c); err != nil {
		return err
	}

	if err := rolebindings.Create(s, c); err != nil {
		return err
	}

	if err := networkpolicies.Create(s, c); err != nil {
		return err
	}

	return nil
}

//DeleteSpace deletes a namespace for a given user
func DeleteSpace(s *model.Space) error {
	c := client.Get()
	go namespaces.Destroy(s, c)
	return nil
}

//ListSpaces returns the spaces for a given user
func ListSpaces(ctx context.Context, u *model.User) ([]*model.Space, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "app.space.listspaces")
	defer span.Finish()

	c := client.Get()
	ns, err := c.CoreV1().Namespaces().List(
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=true", fmt.Sprintf(namespaces.OktetoMemberLabelTemplate, u.ID)),
			FieldSelector: "status.phase=Active",
		},
	)
	if err != nil {
		return nil, err
	}
	spaces := []*model.Space{}
	for _, n := range ns.Items {
		s := namespaces.ToModel(ctx, &n, false)
		spaces = append(spaces, s)
	}
	return spaces, nil
}
