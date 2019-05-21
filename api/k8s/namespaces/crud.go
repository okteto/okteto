package namespaces

import (
	"context"
	"fmt"
	"strings"

	"github.com/okteto/app/api/k8s/client"
	"github.com/okteto/app/api/k8s/serviceaccounts"
	"github.com/okteto/app/api/log"
	"github.com/opentracing/opentracing-go"

	"github.com/okteto/app/api/model"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

// Create creates the namespace for a given space
func Create(s *model.Space, c *kubernetes.Clientset) error {
	log.Debugf("Creating namespace '%s'...", s.ID)
	old, err := c.CoreV1().Namespaces().Get(s.ID, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting kubernetes namespace: %s", err)
	}
	n := translate(s)
	if old.Name == "" {
		_, err := c.CoreV1().Namespaces().Create(n)
		if err != nil {
			return fmt.Errorf("Error creating kubernetes namespace: %s", err)
		}
		log.Debugf("Created namespace '%s'.", s.ID)
	} else {
		_, err := c.CoreV1().Namespaces().Update(n)
		if err != nil {
			return fmt.Errorf("Error updating kubernetes namespace: %s", err)
		}
		log.Debugf("Updated namespace '%s'.", s.ID)
	}
	return nil
}

// GetSpaceByID gets a user by her id
func GetSpaceByID(ctx context.Context, id string, u *model.User) (*model.Space, error) {
	c, err := client.Get()
	if err != nil {
		return nil, fmt.Errorf("error getting k8s client: %s", err)
	}

	ns, err := GetByLabel(ctx, fmt.Sprintf("%s=%s", OktetoIDLabel, id), c)
	if err != nil {
		return nil, err
	}
	if len(ns) == 0 {
		return nil, fmt.Errorf("namespace not found for id: %s", id)
	}
	if len(ns) > 1 {
		return nil, fmt.Errorf("%d namespaces returned for id: %s", len(ns), id)
	}

	s := ToModel(ctx, &ns[0])
	for _, m := range s.Members {
		if m.ID == u.ID {
			return s, nil
		}
	}
	return s, nil
}

//GetByLabel query namespaces by label
func GetByLabel(ctx context.Context, label string, c *kubernetes.Clientset) ([]v1.Namespace, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8s.namespaces.crud.getbylabel")
	defer span.Finish()

	sas, err := c.CoreV1().Namespaces().List(
		metav1.ListOptions{
			LabelSelector: label,
		},
	)
	if err != nil {
		return nil, err
	}
	return sas.Items, nil
}

// Destroy destroys a namespace
func Destroy(s *model.Space, c *kubernetes.Clientset) error {
	log.Infof("destroying namespace '%s'...", s.ID)
	if err := c.CoreV1().Namespaces().Delete(s.ID, &metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("couldn't destroy namespace: %s", err)
		}
	}
	log.Infof("namespace '%s' destroyed", s.ID)
	return nil
}

// ToModel converts a namespace into a model.Space
func ToModel(ctx context.Context, n *v1.Namespace) *model.Space {
	s := &model.Space{
		ID:      n.Name,
		Name:    n.Labels[OktetoNameLabel],
		Members: []model.Member{},
	}
	for l := range n.Labels {
		if strings.HasPrefix(l, "dev.okteto.com/member-") {
			id := strings.SplitAfterN(l, "-", 2)[1]
			owner := false
			if n.Labels[OktetoOwnerLabel] == id {
				owner = true
			}
			uMember, err := serviceaccounts.GetUserByID(ctx, id)
			if err != nil {
				log.Errorf("Error getting user %s", err)
				//TODO: handle error
				continue
			}
			s.Members = append(
				s.Members,
				model.Member{
					ID:       uMember.ID,
					Name:     uMember.Name,
					GithubID: uMember.GithubID,
					Avatar:   uMember.Avatar,
					Owner:    owner,
				},
			)
		}
	}
	return s
}
